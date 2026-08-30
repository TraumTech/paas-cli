// Package operatorinstallerk8s — выходной адаптер: применяет манифест
// оператора СУБД в кластере владельца его же доступом и выдаёт учётной записи
// платформы право из оператора (DB-05).
//
// Манифест применяется server-side apply, как в кластере платформы: повтор
// приводит объекты к манифесту, а не задваивает. CRD применяются раньше
// остального и дожидаются готовности — иначе объекты их типов не примутся.
package operatorinstallerk8s

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"

	clusterprovisionerk8s "github.com/TraumTech/paas-cli/internal/adapters/cluster_provisioner_k8s"
	"github.com/TraumTech/paas-cli/internal/adapters/kubeconfig"
	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

const (
	// fieldManager — от чьего имени применяются объекты; виден в managedFields.
	fieldManager = "paas-cli"
	// rolePrefix — роль и привязка под оператор именуются по типу СУБД:
	// paas-platform-postgres. Отдельно от роли подключения (paas-platform):
	// ту переписывает clusters connect, и право под оператор она бы стёрла.
	rolePrefix = "paas-platform-"

	crdKind = "CustomResourceDefinition"

	crdWaitTimeout      = 60 * time.Second
	operatorWaitTimeout = 3 * time.Minute
	pollInterval        = 2 * time.Second
)

var crdGVR = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}

type Installer struct{}

func New() *Installer { return &Installer{} }

var _ usecases.OperatorInstaller = (*Installer)(nil)

func (i *Installer) AccountName() string {
	return fmt.Sprintf("%s/%s", clusterprovisionerk8s.Namespace, clusterprovisionerk8s.ServiceAccountName)
}

func (i *Installer) Target(kubeconfigPath, contextName string) (*usecases.ClusterTarget, error) {
	config, raw, err := kubeconfig.Load(kubeconfigPath, contextName)
	if err != nil {
		return nil, err
	}
	ca, err := kubeconfig.CACertificate(config)
	if err != nil {
		return nil, err
	}
	return &usecases.ClusterTarget{Endpoint: config.Host, CACertificate: ca, ContextName: raw}, nil
}

func (i *Installer) Objects(manifest string) ([]entities.ManifestObject, error) {
	objects, err := parse(manifest)
	if err != nil {
		return nil, err
	}
	out := make([]entities.ManifestObject, 0, len(objects))
	for _, obj := range objects {
		out = append(out, ref(obj))
	}
	return out, nil
}

func (i *Installer) Install(
	ctx context.Context,
	kubeconfigPath, contextName string,
	op *entities.DatabaseOperator,
) (*entities.OperatorInstallReport, error) {
	objects, err := parse(op.Manifest)
	if err != nil {
		return nil, err
	}
	config, _, err := kubeconfig.Load(kubeconfigPath, contextName)
	if err != nil {
		return nil, err
	}
	c, err := newClients(config)
	if err != nil {
		return nil, err
	}

	report := &entities.OperatorInstallReport{}
	// Сначала CRD: до их готовности объекты их типов сервер не примет.
	crds, rest := split(objects)
	for _, obj := range crds {
		change, err := c.apply(ctx, obj)
		if err != nil {
			return nil, err
		}
		report.Changes = append(report.Changes, change)
	}
	if len(crds) > 0 {
		if err := c.waitEstablished(ctx, crds); err != nil {
			return nil, err
		}
		// Маппер кэширует ресурсы сервера — новые типы он ещё не видел.
		c.mapper.Reset()
	}
	for _, obj := range rest {
		change, err := c.apply(ctx, obj)
		if err != nil {
			return nil, err
		}
		report.Changes = append(report.Changes, change)
	}

	if err := c.waitDeployment(ctx, op.Namespace, op.Deployment); err != nil {
		return nil, err
	}

	// Право платформе — после оператора: пока CRD нет, оно бессмысленно.
	grants, err := c.grant(ctx, op)
	if err != nil {
		return nil, err
	}
	report.Changes = append(report.Changes, grants...)
	return report, nil
}

type clients struct {
	dynamic dynamic.Interface
	typed   kubernetes.Interface
	mapper  *restmapper.DeferredDiscoveryRESTMapper
}

func newClients(config *rest.Config) (*clients, error) {
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("подключиться к кластеру: %w", err)
	}
	typed, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("подключиться к кластеру: %w", err)
	}
	disc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("подключиться к кластеру: %w", err)
	}
	return &clients{
		dynamic: dyn,
		typed:   typed,
		mapper:  restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(disc)),
	}, nil
}

// apply применяет объект server-side и говорит, что с ним стало: ревизия
// объекта до и после — единственный честный признак «менять было нечего».
func (c *clients) apply(ctx context.Context, obj *unstructured.Unstructured) (entities.ObjectChange, error) {
	change := entities.ObjectChange{ManifestObject: ref(obj)}
	gvk := obj.GroupVersionKind()
	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return change, fmt.Errorf("%s %s: кластер не знает такого типа: %w", gvk.Kind, obj.GetName(), err)
	}
	resource := c.dynamic.Resource(mapping.Resource)
	var client dynamic.ResourceInterface = resource
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		client = resource.Namespace(obj.GetNamespace())
	}

	before := ""
	existing, err := client.Get(ctx, obj.GetName(), metav1.GetOptions{})
	switch {
	case err == nil:
		before = existing.GetResourceVersion()
	case apierrors.IsNotFound(err):
	default:
		return change, wrapAccess(err, fmt.Sprintf("прочитать %s %s", gvk.Kind, obj.GetName()))
	}

	applied, err := client.Apply(ctx, obj.GetName(), obj, metav1.ApplyOptions{FieldManager: fieldManager, Force: true})
	if err != nil {
		return change, wrapAccess(err, fmt.Sprintf("применить %s %s", gvk.Kind, obj.GetName()))
	}
	switch {
	case before == "":
		change.Change = entities.ChangeCreated
	case before == applied.GetResourceVersion():
		change.Change = entities.ChangeUnchanged
	default:
		change.Change = entities.ChangeUpdated
	}
	return change, nil
}

func (c *clients) waitEstablished(ctx context.Context, crds []*unstructured.Unstructured) error {
	deadline := time.Now().Add(crdWaitTimeout)
	for _, crd := range crds {
		for {
			current, err := c.dynamic.Resource(crdGVR).Get(ctx, crd.GetName(), metav1.GetOptions{})
			if err != nil {
				return wrapAccess(err, "прочитать CRD "+crd.GetName())
			}
			if conditionTrue(current, "Established") {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("CRD %s не стал готов за %s", crd.GetName(), crdWaitTimeout)
			}
			if err := sleep(ctx, pollInterval); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *clients) waitDeployment(ctx context.Context, namespace, name string) error {
	deadline := time.Now().Add(operatorWaitTimeout)
	for {
		d, err := c.typed.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return wrapAccess(err, "прочитать состояние оператора")
		}
		if err == nil && deploymentReady(d) {
			return nil
		}
		if time.Now().After(deadline) {
			return entities.ErrOperatorNotReady
		}
		if err := sleep(ctx, pollInterval); err != nil {
			return err
		}
	}
}

// grant выдаёт учётной записи платформы право из оператора: своя роль и
// привязка на тип СУБД, приводятся к тому, что платформа просит сейчас.
func (c *clients) grant(ctx context.Context, op *entities.DatabaseOperator) ([]entities.ObjectChange, error) {
	name := rolePrefix + op.Engine
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Rules:      toPolicyRules(op.Rules),
	}
	roleChange := entities.ObjectChange{ManifestObject: entities.ManifestObject{Kind: "ClusterRole", Name: name}}
	existing, err := c.typed.RbacV1().ClusterRoles().Get(ctx, name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		if _, err := c.typed.RbacV1().ClusterRoles().Create(ctx, role, metav1.CreateOptions{}); err != nil {
			return nil, wrapAccess(err, "выдать право платформе")
		}
		roleChange.Change = entities.ChangeCreated
	case err != nil:
		return nil, wrapAccess(err, "прочитать право платформы")
	default:
		roleChange.Change = entities.ChangeUnchanged
		if !equalRules(existing.Rules, role.Rules) {
			existing.Rules = role.Rules
			if _, err := c.typed.RbacV1().ClusterRoles().Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
				return nil, wrapAccess(err, "обновить право платформы")
			}
			roleChange.Change = entities.ChangeUpdated
		}
	}

	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: name},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      clusterprovisionerk8s.ServiceAccountName,
			Namespace: clusterprovisionerk8s.Namespace,
		}},
	}
	bindingChange := entities.ObjectChange{ManifestObject: entities.ManifestObject{Kind: "ClusterRoleBinding", Name: name}, Change: entities.ChangeCreated}
	_, err = c.typed.RbacV1().ClusterRoleBindings().Create(ctx, binding, metav1.CreateOptions{})
	switch {
	case apierrors.IsAlreadyExists(err):
		bindingChange.Change = entities.ChangeUnchanged
	case err != nil:
		return nil, wrapAccess(err, "привязать право к учётной записи платформы")
	}
	return []entities.ObjectChange{roleChange, bindingChange}, nil
}

// parse разбирает многодокументный YAML в объекты; пустые документы
// пропускаются. Объект без kind или имени — сломанный манифест.
func parse(manifest string) ([]*unstructured.Unstructured, error) {
	decoder := utilyaml.NewYAMLOrJSONDecoder(strings.NewReader(manifest), 4096)
	var objects []*unstructured.Unstructured
	for {
		obj := &unstructured.Unstructured{}
		err := decoder.Decode(obj)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", entities.ErrOperatorManifestBroken, err)
		}
		if len(obj.Object) == 0 {
			continue
		}
		if obj.GetKind() == "" || obj.GetName() == "" {
			return nil, fmt.Errorf("%w: объект без kind или имени", entities.ErrOperatorManifestBroken)
		}
		objects = append(objects, obj)
	}
	if len(objects) == 0 {
		return nil, fmt.Errorf("%w: пусто", entities.ErrOperatorManifestBroken)
	}
	return objects, nil
}

// split выделяет CRD, сохраняя порядок манифеста в обеих частях: в остатке
// namespace'ы идут раньше того, что в них лежит, — как и в самом релизе.
func split(objects []*unstructured.Unstructured) (crds, rest []*unstructured.Unstructured) {
	for _, obj := range objects {
		if obj.GetKind() == crdKind {
			crds = append(crds, obj)
		} else {
			rest = append(rest, obj)
		}
	}
	return crds, rest
}

func ref(obj *unstructured.Unstructured) entities.ManifestObject {
	return entities.ManifestObject{Kind: obj.GetKind(), Namespace: obj.GetNamespace(), Name: obj.GetName()}
}

func conditionTrue(obj *unstructured.Unstructured, conditionType string) bool {
	conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	for _, raw := range conditions {
		condition, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if condition["type"] == conditionType && condition["status"] == "True" {
			return true
		}
	}
	return false
}

func deploymentReady(d *appsv1.Deployment) bool {
	want := int32(1)
	if d.Spec.Replicas != nil {
		want = *d.Spec.Replicas
	}
	return d.Status.ObservedGeneration >= d.Generation &&
		d.Status.UpdatedReplicas == want &&
		d.Status.AvailableReplicas == want
}

func toPolicyRules(rules []entities.AccessRule) []rbacv1.PolicyRule {
	out := make([]rbacv1.PolicyRule, 0, len(rules))
	for _, rule := range rules {
		out = append(out, rbacv1.PolicyRule{APIGroups: rule.APIGroups, Resources: rule.Resources, Verbs: rule.Verbs})
	}
	return out
}

func equalRules(a, b []rbacv1.PolicyRule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !equalSet(a[i].APIGroups, b[i].APIGroups) || !equalSet(a[i].Resources, b[i].Resources) || !equalSet(a[i].Verbs, b[i].Verbs) {
			return false
		}
	}
	return true
}

func equalSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x, y := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// wrapAccess переводит отказ по правам в понятную причину: установка оператора
// — административная операция, и чаще всего не хватает именно этого.
func wrapAccess(err error, action string) error {
	if err == nil {
		return nil
	}
	if apierrors.IsForbidden(err) {
		return fmt.Errorf("%w (%s)", entities.ErrOperatorInstallDenied, action)
	}
	return fmt.Errorf("%s: %w", action, err)
}
