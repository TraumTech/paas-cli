// Package clusterprovisionerk8s — выходной адаптер: заводит в кластере
// владельца учётную запись для платформы, пользуясь его локальным доступом.
//
// Клиентская библиотека Kubernetes здесь нужна не ради вызовов (API обычный
// HTTP), а ради аутентификации: в kubeconfig может лежать exec-плагин облака,
// клиентский сертификат, OIDC или impersonation. Разбирать это самостоятельно
// значит воспроизводить чужой формат и чужой протокол.
package clusterprovisionerk8s

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

const (
	// Имена того, что команда заводит в чужом кластере. Постоянные и говорящие:
	// владелец должен узнавать их у себя и понимать, откуда они взялись.
	namespace          = "kube-system"
	serviceAccountName = "paas-platform"
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
	return fmt.Sprintf("%s/%s", namespace, serviceAccountName)
}

func (p *Provisioner) Target(kubeconfig, contextName string) (*usecases.ClusterTarget, error) {
	config, raw, err := loadConfig(kubeconfig, contextName)
	if err != nil {
		return nil, err
	}
	ca, err := caCertificate(config)
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
	kubeconfig, contextName string,
	rules []entities.AccessRule,
) (*entities.ClusterCredential, error) {
	config, _, err := loadConfig(kubeconfig, contextName)
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

	ca, err := caCertificate(config)
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
		ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace},
	}
	_, err := client.CoreV1().ServiceAccounts(namespace).Create(ctx, account, metav1.CreateOptions{})
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
			Name:      serviceAccountName,
			Namespace: namespace,
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
			Namespace:   namespace,
			Annotations: map[string]string{corev1.ServiceAccountNameKey: serviceAccountName},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
	_, err := client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return "", wrapAccess(err, "завести секрет с токеном")
	}

	deadline := time.Now().Add(tokenWaitTimeout)
	for {
		stored, err := client.CoreV1().Secrets(namespace).Get(ctx, tokenSecretName, metav1.GetOptions{})
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

func loadConfig(kubeconfig, contextName string) (*rest.Config, string, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		rules.ExplicitPath = kubeconfig
	}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	raw, err := clientConfig.RawConfig()
	if err != nil {
		return nil, "", fmt.Errorf("прочитать kubeconfig: %w", err)
	}
	selected := contextName
	if selected == "" {
		selected = raw.CurrentContext
	}
	if selected == "" {
		return nil, "", entities.ErrNoKubeContext
	}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("прочитать доступ к кластеру: %w", err)
	}
	return config, selected, nil
}

// caCertificate возвращает сертификат кластера в PEM: платформе он нужен,
// чтобы опознать кластер — у облачных он самоподписанный.
func caCertificate(config *rest.Config) (string, error) {
	if len(config.CAData) > 0 {
		return string(config.CAData), nil
	}
	if config.CAFile == "" {
		return "", fmt.Errorf("в доступе к кластеру нет его сертификата")
	}
	data, err := os.ReadFile(config.CAFile)
	if err != nil {
		return "", fmt.Errorf("прочитать сертификат кластера: %w", err)
	}
	return string(data), nil
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
