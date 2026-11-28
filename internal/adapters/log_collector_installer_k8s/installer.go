// Package logcollectorinstallerk8s — выходной адаптер: ставит в кластер
// приложений сборщик вывода сервисов доступом владельца (TRA-559).
//
// Манифест зашит в CLI: в нём нет ничего, что знает только платформа, — адрес
// приёма и ключ приходят параметрами команды.
package logcollectorinstallerk8s

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"text/template"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/TraumTech/paas-cli/internal/adapters/kubeconfig"
	operatorinstallerk8s "github.com/TraumTech/paas-cli/internal/adapters/operator_installer_k8s"
	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

const (
	namespace   = "stapel-logs"
	daemonSet   = "collector"
	waitTimeout = 3 * time.Minute
	pollEvery   = 2 * time.Second
)

//go:embed collector.yaml
var manifestSource string

var manifestTemplate = template.Must(template.New("collector").Parse(manifestSource))

type Installer struct {
	objects *operatorinstallerk8s.Installer
}

func New() *Installer { return &Installer{objects: operatorinstallerk8s.New()} }

var _ usecases.LogCollectorInstaller = (*Installer)(nil)

func (i *Installer) Target(kubeconfigPath, contextName string) (*usecases.ClusterTarget, error) {
	return i.objects.Target(kubeconfigPath, contextName)
}

func (i *Installer) Objects(collector entities.LogCollector) ([]entities.ManifestObject, error) {
	manifest, err := render(collector)
	if err != nil {
		return nil, err
	}
	return i.objects.Objects(manifest)
}

func (i *Installer) Install(ctx context.Context, kubeconfigPath, contextName string, collector entities.LogCollector) (*entities.OperatorInstallReport, error) {
	manifest, err := render(collector)
	if err != nil {
		return nil, err
	}
	report, err := i.objects.Apply(ctx, kubeconfigPath, contextName, manifest)
	var denied *operatorinstallerk8s.AccessDenied
	if errors.As(err, &denied) {
		return nil, fmt.Errorf("%w (%s): %s", entities.ErrLogCollectorInstallDenied, denied.Action, denied.Reason)
	}
	if err != nil {
		return nil, err
	}

	config, _, err := kubeconfig.Load(kubeconfigPath, contextName)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("подключиться к кластеру: %w", err)
	}
	if err := waitReady(ctx, client); err != nil {
		return nil, err
	}
	return report, nil
}

func render(collector entities.LogCollector) (string, error) {
	// Под перечитывает ключ и адрес только при старте: их смена должна
	// перекатить сборщик, поэтому отпечаток настроек лежит в шаблоне пода.
	sum := sha256.Sum256([]byte(collector.PushURL + "\n" + collector.Key))
	var out bytes.Buffer
	err := manifestTemplate.Execute(&out, map[string]string{
		"PushURL":   strconv.Quote(collector.PushURL),
		"KeyBase64": base64.StdEncoding.EncodeToString([]byte(collector.Key)),
		"Checksum":  hex.EncodeToString(sum[:8]),
	})
	if err != nil {
		return "", fmt.Errorf("собрать манифест сборщика: %w", err)
	}
	return out.String(), nil
}

func waitReady(ctx context.Context, client kubernetes.Interface) error {
	deadline := time.Now().Add(waitTimeout)
	for {
		ds, err := client.AppsV1().DaemonSets(namespace).Get(ctx, daemonSet, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("прочитать состояние сборщика: %w", err)
		}
		if daemonSetReady(ds) {
			return nil
		}
		if time.Now().After(deadline) {
			return entities.ErrLogCollectorNotReady
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollEvery):
		}
	}
}

func daemonSetReady(ds *appsv1.DaemonSet) bool {
	return ds.Status.ObservedGeneration >= ds.Generation &&
		ds.Status.DesiredNumberScheduled > 0 &&
		ds.Status.UpdatedNumberScheduled == ds.Status.DesiredNumberScheduled &&
		ds.Status.NumberAvailable == ds.Status.DesiredNumberScheduled
}
