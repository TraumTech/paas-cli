// Package kubeconfig — локальный доступ владельца к кластеру. Клиентская
// библиотека Kubernetes здесь нужна не ради вызовов (API обычный HTTP), а ради
// аутентификации: в kubeconfig может лежать exec-плагин облака, клиентский
// сертификат, OIDC или impersonation. Разбирать это самостоятельно значит
// воспроизводить чужой формат и чужой протокол.
package kubeconfig

import (
	"fmt"
	"os"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// Load читает доступ к кластеру из kubeconfig (пусто — KUBECONFIG, затем
// ~/.kube/config) и контекста (пусто — текущий). Возвращает и имя выбранного
// контекста — оно показывается владельцу.
func Load(kubeconfig, contextName string) (*rest.Config, string, error) {
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

// CACertificate возвращает сертификат кластера в PEM: платформе он нужен,
// чтобы опознать кластер — у облачных он самоподписанный.
func CACertificate(config *rest.Config) (string, error) {
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
