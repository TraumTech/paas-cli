// Package databaseaccessk8s — выходной адаптер: личный вход владельца в СУБД
// CloudNativePG его же доступом к кластеру. Роль и пароль заводятся
// манифестами оператора, проброс порта идёт через API кластера — наружу у
// СУБД ничего не открывается.
package databaseaccessk8s

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/TraumTech/paas-cli/internal/adapters/kubeconfig"
	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

const (
	// rolesFieldManager — тот же владелец списка ролей, что у платформы
	// (paas-workloads): список у оператора неделимый, и при двух владельцах
	// каждый перезаписывал бы чужие роли. Платформа дописывает свои к
	// прочитанным, поэтому личная роль переживает заказ новых баз.
	rolesFieldManager = "paas-platform-roles"
	secretPrefix      = "paas-user-"
	postgresPort      = 5432

	roleWaitTimeout = 90 * time.Second
	pollInterval    = 2 * time.Second
)

// personalRoleGrants — личная роль видит схемы и данные всех баз, но ничего
// не меняет: доступ для наблюдения, а не для правки баз сервисов.
var personalRoleGrants = []any{"pg_read_all_data", "pg_monitor"}

var (
	clustersGVR  = schema.GroupVersionResource{Group: "postgresql.cnpg.io", Version: "v1", Resource: "clusters"}
	databasesGVR = schema.GroupVersionResource{Group: "postgresql.cnpg.io", Version: "v1", Resource: "databases"}
)

type Access struct{}

func New() *Access { return &Access{} }

var _ usecases.DatabaseAccess = (*Access)(nil)

func (a *Access) Connect(kubeconfigPath, contextName string) (usecases.DatabaseCluster, error) {
	config, selected, err := kubeconfig.Load(kubeconfigPath, contextName)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("клиент кластера: %w", err)
	}
	core, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("клиент кластера: %w", err)
	}
	return &cluster{config: config, context: selected, dyn: dyn, core: core}, nil
}

type cluster struct {
	config  *rest.Config
	context string
	dyn     dynamic.Interface
	core    kubernetes.Interface
}

func (c *cluster) ContextName() string { return c.context }

func (c *cluster) Find(ctx context.Context, name, namespace string) (*entities.DatabaseServer, error) {
	list, err := c.dyn.Resource(clustersGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, accessError("прочитать СУБД", err)
	}
	var found []unstructured.Unstructured
	for _, item := range list.Items {
		if item.GetName() == name {
			found = append(found, item)
		}
	}
	switch len(found) {
	case 0:
		return nil, entities.ErrDatabaseServerNotFound
	case 1:
	default:
		namespaces := make([]string, 0, len(found))
		for _, item := range found {
			namespaces = append(namespaces, item.GetNamespace())
		}
		sort.Strings(namespaces)
		return nil, &entities.AmbiguousDatabaseServerError{Name: name, Namespaces: namespaces}
	}

	obj := found[0]
	server := &entities.DatabaseServer{Namespace: obj.GetNamespace(), Name: name, Roles: roleNames(managedRoles(obj))}
	databases, err := c.dyn.Resource(databasesGVR).Namespace(server.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, accessError("прочитать базы СУБД", err)
	}
	for _, db := range databases.Items {
		owner, _, _ := unstructured.NestedString(db.Object, "spec", "cluster", "name")
		dbName, _, _ := unstructured.NestedString(db.Object, "spec", "name")
		if owner == name && dbName != "" {
			server.Databases = append(server.Databases, dbName)
		}
	}
	sort.Strings(server.Databases)
	return server, nil
}

func (c *cluster) EnsureRole(ctx context.Context, server entities.DatabaseServer, role string) error {
	secret := secretName(role)
	if err := c.ensureSecret(ctx, server.Namespace, secret, role); err != nil {
		return err
	}

	obj, err := c.dyn.Resource(clustersGVR).Namespace(server.Namespace).Get(ctx, server.Name, metav1.GetOptions{})
	if err != nil {
		return accessError("прочитать СУБД", err)
	}
	body, err := json.Marshal(map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1", "kind": "Cluster",
		"metadata": map[string]any{"name": server.Name, "namespace": server.Namespace},
		"spec":     map[string]any{"managed": map[string]any{"roles": withPersonalRole(managedRoles(*obj), role, secret)}},
	})
	if err != nil {
		return err
	}
	force := true
	_, err = c.dyn.Resource(clustersGVR).Namespace(server.Namespace).Patch(ctx, server.Name, types.ApplyPatchType, body,
		metav1.PatchOptions{FieldManager: rolesFieldManager, Force: &force})
	if err != nil {
		return accessError("завести роль", err)
	}
	return c.waitRole(ctx, server, role)
}

func (c *cluster) ensureSecret(ctx context.Context, namespace, name, role string) error {
	_, err := c.core.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return accessError("прочитать пароль роли", err)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: namespace,
			// Метка reload — оператор подхватывает смену пароля без рестарта.
			Labels: map[string]string{"cnpg.io/reload": "true", "app.kubernetes.io/managed-by": "paas-cli"},
		},
		Type:       corev1.SecretTypeBasicAuth,
		StringData: map[string]string{corev1.BasicAuthUsernameKey: role, corev1.BasicAuthPasswordKey: rand.Text()},
	}
	if _, err := c.core.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		return accessError("сохранить пароль роли", err)
	}
	return nil
}

func (c *cluster) waitRole(ctx context.Context, server entities.DatabaseServer, role string) error {
	ctx, cancel := context.WithTimeout(ctx, roleWaitTimeout)
	defer cancel()
	for {
		obj, err := c.dyn.Resource(clustersGVR).Namespace(server.Namespace).Get(ctx, server.Name, metav1.GetOptions{})
		if err == nil {
			ready, reason := roleState(*obj, role)
			if ready {
				return nil
			}
			if reason != "" {
				return &entities.DatabaseRoleRejectedError{Role: role, Reason: reason}
			}
		}
		select {
		case <-ctx.Done():
			return entities.ErrDatabaseRoleNotReady
		case <-time.After(pollInterval):
		}
	}
}

func (c *cluster) Credentials(ctx context.Context, server entities.DatabaseServer, role string) (*entities.DatabaseCredentials, error) {
	secret, err := c.core.CoreV1().Secrets(server.Namespace).Get(ctx, secretName(role), metav1.GetOptions{})
	if err != nil {
		return nil, accessError("прочитать пароль роли", err)
	}
	return &entities.DatabaseCredentials{User: role, Password: string(secret.Data[corev1.BasicAuthPasswordKey])}, nil
}

func (c *cluster) Forward(ctx context.Context, server entities.DatabaseServer, port int, ready func()) error {
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	probe, err := net.Listen("tcp", address)
	if err != nil {
		return entities.ErrLocalPortBusy
	}
	_ = probe.Close()

	pod, err := c.primary(ctx, server)
	if err != nil {
		return err
	}
	dialer, err := c.dialer(server.Namespace, pod)
	if err != nil {
		return err
	}

	stop, readyCh := make(chan struct{}), make(chan struct{})
	pf, err := portforward.NewOnAddresses(dialer, []string{"127.0.0.1"}, []string{fmt.Sprintf("%d:%d", port, postgresPort)},
		stop, readyCh, io.Discard, io.Discard)
	if err != nil {
		return err
	}

	// ready вызывается до возврата Forward: вызывающий читает и пишет своё
	// состояние без блокировок.
	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-readyCh:
			ready()
		case <-done:
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
			close(stop)
		case <-done:
		}
	}()
	err = pf.ForwardPorts()
	close(done)
	wg.Wait()
	return err
}

// primary — под основного инстанса: туда пишут сервисы, и туда же смотрит
// сервис <имя>-rw.
func (c *cluster) primary(ctx context.Context, server entities.DatabaseServer) (string, error) {
	obj, err := c.dyn.Resource(clustersGVR).Namespace(server.Namespace).Get(ctx, server.Name, metav1.GetOptions{})
	if err != nil {
		return "", accessError("прочитать СУБД", err)
	}
	name, _, _ := unstructured.NestedString(obj.Object, "status", "currentPrimary")
	if name == "" {
		return "", entities.ErrDatabasePrimaryNotReady
	}
	pod, err := c.core.CoreV1().Pods(server.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", accessError("прочитать под СУБД", err)
	}
	if pod.Status.Phase != corev1.PodRunning {
		return "", entities.ErrDatabasePrimaryNotReady
	}
	return name, nil
}

// dialer — как у kubectl port-forward: WebSocket, а если сервер его не
// поддерживает — SPDY. У обоих пинги, так что мёртвая связь замечается.
func (c *cluster) dialer(namespace, pod string) (httpstream.Dialer, error) {
	url := c.core.CoreV1().RESTClient().Post().Resource("pods").Namespace(namespace).Name(pod).SubResource("portforward").URL()
	transport, upgrader, err := spdy.RoundTripperFor(c.config)
	if err != nil {
		return nil, err
	}
	spdyDialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, url)
	wsDialer, err := portforward.NewSPDYOverWebsocketDialer(url, c.config)
	if err != nil {
		return nil, err
	}
	return portforward.NewFallbackDialer(wsDialer, spdyDialer, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	}), nil
}

func managedRoles(obj unstructured.Unstructured) []any {
	roles, _, _ := unstructured.NestedSlice(obj.Object, "spec", "managed", "roles")
	return roles
}

func roleNames(roles []any) []string {
	names := make([]string, 0, len(roles))
	for _, r := range roles {
		if m, ok := r.(map[string]any); ok {
			if name, ok := m["name"].(string); ok {
				names = append(names, name)
			}
		}
	}
	return names
}

// withPersonalRole — список ролей CR с личной ролью; роли платформы и прочие
// остаются как есть, одноимённая заменяется.
func withPersonalRole(existing []any, role, secret string) []any {
	out := make([]any, 0, len(existing)+1)
	for _, r := range existing {
		if m, ok := r.(map[string]any); ok && m["name"] == role {
			continue
		}
		out = append(out, r)
	}
	return append(out, map[string]any{
		"name":           role,
		"ensure":         "present",
		"login":          true,
		"inRoles":        personalRoleGrants,
		"passwordSecret": map[string]any{"name": secret},
		"comment":        "личный вход владельца: paas-cli databases forward",
	})
}

// roleState читает managedRolesStatus оператора: применена ли роль, а если
// нет — отказал ли оператор и почему.
func roleState(obj unstructured.Unstructured, role string) (ready bool, reason string) {
	reconciled, _, _ := unstructured.NestedStringSlice(obj.Object, "status", "managedRolesStatus", "byStatus", "reconciled")
	for _, name := range reconciled {
		if name == role {
			return true, ""
		}
	}
	errs, _, _ := unstructured.NestedStringSlice(obj.Object, "status", "managedRolesStatus", "cannotReconcile", role)
	return false, strings.Join(errs, "; ")
}

// secretName — имя секрета Kubernetes не допускает «_», роль в Postgres — да.
func secretName(role string) string {
	return secretPrefix + strings.ReplaceAll(role, "_", "-")
}

func accessError(action string, err error) error {
	if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) {
		return entities.ErrDatabaseAccessDenied
	}
	if apierrors.IsNotFound(err) && strings.Contains(err.Error(), "the server could not find the requested resource") {
		return errors.New("в кластере нет оператора CloudNativePG — поставьте его: paas-cli databases operators install --engine postgres")
	}
	return fmt.Errorf("%s: %w", action, err)
}
