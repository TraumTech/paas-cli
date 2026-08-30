// Package clusterprovisionerk8s — выходной адаптер: заводит в кластере
// владельца учётную запись для платформы, пользуясь его локальным доступом
// (см. adapters/kubeconfig — почему через клиентскую библиотеку).
package clusterprovisionerk8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/TraumTech/paas-cli/internal/adapters/kubeconfig"
	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

const (
	// Имена того, что команда заводит в чужом кластере. Постоянные и говорящие:
	// владелец должен узнавать их у себя и понимать, откуда они взялись.
	// Учётная запись экспортирована: к ней привязывают и права под оператор
	// СУБД (DB-05).
	Namespace          = "kube-system"
	ServiceAccountName = "paas-platform"
	clusterRoleName    = "paas-platform"
	bindingName        = "paas-platform"
	tokenSecretName    = "paas-platform-token"

	// Токен в секрете появляется не мгновенно — его дописывает контроллер.
	tokenWaitTimeout  = 30 * time.Second
	tokenWaitInterval = 500 * time.Millisecond
)

type Provisioner struct{}

func New() *Provisioner { return &Provisioner{} }

var _ usecases.ClusterProvisioner = (*Provisioner)(nil)

func (p *Provisioner) AccountName() string {
	return fmt.Sprintf("%s/%s", Namespace, ServiceAccountName)
}

func (p *Provisioner) Target(kubeconfigPath, contextName string) (*usecases.ClusterTarget, error) {
	config, raw, err := kubeconfig.Load(kubeconfigPath, contextName)
	if err != nil {
		return nil, err
	}
	ca, err := kubeconfig.CACertificate(config)
	if err != nil {
		return nil, err
	}
	return &usecases.ClusterTarget{
		Endpoint:      config.Host,
		CACertificate: ca,
		ContextName:   raw,
	}, nil
}

// Provision заводит учётную запись, роль, привязку и секрет с токеном.
// Идемпотентна: уже существующее переиспользуется, права роли приводятся к
// запрошенным — повтор после неудачи не задваивает и не оставляет мусора.
func (p *Provisioner) Provision(
	ctx context.Context,
	kubeconfigPath, contextName string,
	rules []entities.AccessRule,
) (*entities.ClusterCredential, error) {
	config, _, err := kubeconfig.Load(kubeconfigPath, contextName)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("подключиться к кластеру: %w", err)
	}

	if err := p.applyServiceAccount(ctx, client); err != nil {
		return nil, err
	}
	if err := p.applyClusterRole(ctx, client, rules); err != nil {
		return nil, err
	}
	if err := p.applyBinding(ctx, client); err != nil {
		return nil, err
	}
	token, err := p.ensureToken(ctx, client)
	if err != nil {
		return nil, err
	}

	ca, err := kubeconfig.CACertificate(config)
	if err != nil {
		return nil, err
	}
	return &entities.ClusterCredential{
		Endpoint:      config.Host,
		CACertificate: ca,
		Token:         token,
	}, nil
}

func (p *Provisioner) applyServiceAccount(ctx context.Context, client kubernetes.Interface) error {
	account := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: ServiceAccountName, Namespace: Namespace},
	}
	_, err := client.CoreV1().ServiceAccounts(Namespace).Create(ctx, account, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return wrapAccess(err, "завести учётную запись")
}

func (p *Provisioner) applyClusterRole(ctx context.Context, client kubernetes.Interface, rules []entities.AccessRule) error {
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName},
		Rules:      toPolicyRules(rules),
	}
	_, err := client.RbacV1().ClusterRoles().Create(ctx, role, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		// Роль могла остаться от прошлого подключения с другим набором прав —
		// приводим к тому, что платформа просит сейчас.
		_, err = client.RbacV1().ClusterRoles().Update(ctx, role, metav1.UpdateOptions{})
	}
	return wrapAccess(err, "выдать права платформе")
}

func (p *Provisioner) applyBinding(ctx context.Context, client kubernetes.Interface) error {
	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: bindingName},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      ServiceAccountName,
			Namespace: Namespace,
		}},
	}
	_, err := client.RbacV1().ClusterRoleBindings().Create(ctx, binding, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return wrapAccess(err, "привязать права к учётной записи")
}

// ensureToken заводит секрет с долгоживущим токеном: сам по себе у учётной
// записи он с Kubernetes 1.24 не появляется.
func (p *Provisioner) ensureToken(ctx context.Context, client kubernetes.Interface) (string, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        tokenSecretName,
			Namespace:   Namespace,
			Annotations: map[string]string{corev1.ServiceAccountNameKey: ServiceAccountName},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
	_, err := client.CoreV1().Secrets(Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return "", wrapAccess(err, "завести секрет с токеном")
	}

	deadline := time.Now().Add(tokenWaitTimeout)
	for {
		stored, err := client.CoreV1().Secrets(Namespace).Get(ctx, tokenSecretName, metav1.GetOptions{})
		if err != nil {
			return "", wrapAccess(err, "прочитать токен учётной записи")
		}
		if token := stored.Data[corev1.ServiceAccountTokenKey]; len(token) > 0 {
			return string(token), nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("токен учётной записи не появился за %s", tokenWaitTimeout)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(tokenWaitInterval):
		}
	}
}

func toPolicyRules(rules []entities.AccessRule) []rbacv1.PolicyRule {
	out := make([]rbacv1.PolicyRule, 0, len(rules))
	for _, rule := range rules {
		out = append(out, rbacv1.PolicyRule{
			APIGroups: rule.APIGroups,
			Resources: rule.Resources,
			Verbs:     rule.Verbs,
		})
	}
	return out
}

// wrapAccess переводит отказ по правам в понятную причину: чаще всего у
// владельца просто нет прав раздавать права.
func wrapAccess(err error, action string) error {
	if err == nil {
		return nil
	}
	if apierrors.IsForbidden(err) {
		return fmt.Errorf("%w (%s)", entities.ErrClusterAccessDenied, action)
	}
	return fmt.Errorf("%s: %w", action, err)
}
