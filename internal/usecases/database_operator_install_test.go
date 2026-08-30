package usecases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type installFixture struct {
	operators *MockDatabaseOperatorSource
	clusters  *MockClusterDirectory
	installer *MockOperatorInstaller
	uc        *InstallOperatorUseCase
}

func newInstallFixture(t *testing.T) *installFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := &installFixture{
		operators: NewMockDatabaseOperatorSource(ctrl),
		clusters:  NewMockClusterDirectory(ctrl),
		installer: NewMockOperatorInstaller(ctrl),
	}
	f.uc = NewInstallOperator(f.operators, f.clusters, f.installer)
	return f
}

var (
	cnpg = &entities.DatabaseOperator{Engine: "postgres", Name: "CloudNativePG", Version: "1.30.0", Manifest: "kind: Namespace\n",
		Rules: []entities.AccessRule{{APIGroups: []string{"postgresql.cnpg.io"}, Resources: []string{"clusters"}, Verbs: []string{"create"}}}}
	connected = []entities.ConnectedCluster{{Name: "yc-dev", Endpoint: "https://c/"}}
	objects   = []entities.ManifestObject{{Kind: "Namespace", Name: "cnpg-system"}}
)

func agreeInstall(InstallOperatorPlan) (bool, error)  { return true, nil }
func refuseInstall(InstallOperatorPlan) (bool, error) { return false, nil }

func TestInstallOperator(t *testing.T) {
	f := newInstallFixture(t)
	ctx := context.Background()
	report := &entities.OperatorInstallReport{Changes: []entities.ObjectChange{{Change: entities.ChangeCreated}}}

	gomock.InOrder(
		// Оператор спрашивается до того, как в кластере что-то меняется.
		f.operators.EXPECT().Operator(ctx, "postgres").Return(cnpg, nil),
		f.installer.EXPECT().Target("", "ctx").Return(&ClusterTarget{Endpoint: "https://c"}, nil),
		f.clusters.EXPECT().ListClusters(ctx).Return(connected, nil),
		f.installer.EXPECT().Objects(cnpg.Manifest).Return(objects, nil),
		f.installer.EXPECT().Install(ctx, "", "ctx", cnpg).Return(report, nil),
	)
	f.installer.EXPECT().AccountName().Return("kube-system/paas-platform")

	var shown InstallOperatorPlan
	result, err := f.uc.Execute(ctx, InstallOperatorInput{Engine: "postgres", Context: "ctx"}, func(plan InstallOperatorPlan) (bool, error) {
		shown = plan
		return true, nil
	})

	require.NoError(t, err)
	assert.Equal(t, "yc-dev", result.ClusterName)
	assert.Equal(t, *report, result.Report)
	// План — из того, что отдала платформа и что нашлось в манифесте.
	assert.Equal(t, "yc-dev", shown.ClusterName)
	assert.Equal(t, cnpg.Rules, shown.Operator.Rules)
	assert.Equal(t, objects, shown.Objects)
}

// Владелец не согласился — в кластере ничего не меняется.
func TestInstallOperatorRefused(t *testing.T) {
	f := newInstallFixture(t)
	ctx := context.Background()
	f.operators.EXPECT().Operator(ctx, "postgres").Return(cnpg, nil)
	f.installer.EXPECT().Target("", "").Return(&ClusterTarget{Endpoint: "https://c"}, nil)
	f.clusters.EXPECT().ListClusters(ctx).Return(connected, nil)
	f.installer.EXPECT().Objects(cnpg.Manifest).Return(objects, nil)
	f.installer.EXPECT().AccountName().Return("kube-system/paas-platform")
	// Install не ожидается.

	_, err := f.uc.Execute(ctx, InstallOperatorInput{Engine: "postgres"}, refuseInstall)

	assert.ErrorIs(t, err, entities.ErrCancelled)
}

// Оператор ставится только в подключённый кластер: право достаётся учётной
// записи, которую заводит подключение, — без него ставить не для кого.
func TestInstallOperatorRequiresConnectedCluster(t *testing.T) {
	f := newInstallFixture(t)
	ctx := context.Background()
	f.operators.EXPECT().Operator(ctx, "postgres").Return(cnpg, nil)
	f.installer.EXPECT().Target("", "").Return(&ClusterTarget{Endpoint: "https://other"}, nil)
	f.clusters.EXPECT().ListClusters(ctx).Return(connected, nil)

	_, err := f.uc.Execute(ctx, InstallOperatorInput{Engine: "postgres"}, agreeInstall)

	assert.ErrorIs(t, err, entities.ErrClusterNotConnected)
}

func TestInstallOperatorUnknownEngine(t *testing.T) {
	f := newInstallFixture(t)
	ctx := context.Background()
	f.operators.EXPECT().Operator(ctx, "oracle").Return(nil, entities.ErrUnknownEngine)

	_, err := f.uc.Execute(ctx, InstallOperatorInput{Engine: "oracle"}, agreeInstall)

	assert.ErrorIs(t, err, entities.ErrUnknownEngine)
}

func TestInstallOperatorEmptyEngine(t *testing.T) {
	f := newInstallFixture(t)

	_, err := f.uc.Execute(context.Background(), InstallOperatorInput{}, agreeInstall)

	assert.ErrorIs(t, err, entities.ErrEmptyEngine)
}
