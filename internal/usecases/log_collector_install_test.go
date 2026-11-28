package usecases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// TRA-559/AC-4: сборщик ставится с ключом кластера и адресом платформы.
func TestInstallLogCollector(t *testing.T) {
	ctrl := gomock.NewController(t)
	installer := NewMockLogCollectorInstaller(ctrl)
	collector := entities.LogCollector{Key: "k", PushURL: entities.DefaultLogPushURL}
	objects := []entities.ManifestObject{{Kind: "DaemonSet", Namespace: "stapel-logs", Name: "collector"}}
	report := &entities.OperatorInstallReport{Changes: []entities.ObjectChange{{ManifestObject: objects[0], Change: entities.ChangeCreated}}}

	installer.EXPECT().Target("cfg", "apps").Return(&ClusterTarget{Endpoint: "https://apps:6443"}, nil)
	installer.EXPECT().Objects(collector).Return(objects, nil)
	installer.EXPECT().Install(gomock.Any(), "cfg", "apps", collector).Return(report, nil)

	var shown InstallLogCollectorPlan
	result, err := NewInstallLogCollector(installer).Execute(context.Background(),
		InstallLogCollectorInput{Key: " k\n", Context: "apps", Kubeconfig: "cfg"},
		func(plan InstallLogCollectorPlan) (bool, error) { shown = plan; return true, nil })
	require.NoError(t, err)
	require.Equal(t, "https://apps:6443", result.Endpoint)
	require.Equal(t, *report, result.Report)
	require.Equal(t, InstallLogCollectorPlan{Endpoint: "https://apps:6443", PushURL: entities.DefaultLogPushURL, Objects: objects}, shown)
}

func TestInstallLogCollectorRequiresKey(t *testing.T) {
	installer := NewMockLogCollectorInstaller(gomock.NewController(t))
	_, err := NewInstallLogCollector(installer).Execute(context.Background(), InstallLogCollectorInput{Key: "  "},
		func(InstallLogCollectorPlan) (bool, error) { return true, nil })
	require.ErrorIs(t, err, entities.ErrEmptyLogKey)
}

// Отказ на подтверждении ничего не меняет в кластере.
func TestInstallLogCollectorCancelled(t *testing.T) {
	installer := NewMockLogCollectorInstaller(gomock.NewController(t))
	installer.EXPECT().Target("", "").Return(&ClusterTarget{Endpoint: "https://apps:6443"}, nil)
	installer.EXPECT().Objects(gomock.Any()).Return(nil, nil)

	_, err := NewInstallLogCollector(installer).Execute(context.Background(), InstallLogCollectorInput{Key: "k"},
		func(InstallLogCollectorPlan) (bool, error) { return false, nil })
	require.ErrorIs(t, err, entities.ErrCancelled)
}
