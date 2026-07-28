package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type connectFixture struct {
	access      *MockClusterAccessSource
	provisioner *MockClusterProvisioner
	registrar   *MockClusterRegistrar
	uc          *ConnectClusterUseCase
}

func newConnectFixture(t *testing.T) *connectFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := &connectFixture{
		access:      NewMockClusterAccessSource(ctrl),
		provisioner: NewMockClusterProvisioner(ctrl),
		registrar:   NewMockClusterRegistrar(ctrl),
	}
	f.uc = NewConnectCluster(f.access, f.provisioner, f.registrar)
	return f
}

var rules = []entities.AccessRule{{
	APIGroups: []string{""}, Resources: []string{"namespaces"}, Verbs: []string{"create"},
}}

func agree(ConnectClusterPlan) (bool, error)  { return true, nil }
func refuse(ConnectClusterPlan) (bool, error) { return false, nil }

func TestConnectCluster(t *testing.T) {
	f := newConnectFixture(t)
	ctx := context.Background()

	gomock.InOrder(
		// Права спрашиваются до того, как в кластере что-то меняется.
		f.access.EXPECT().RequiredAccess(ctx).Return(rules, nil),
		f.provisioner.EXPECT().Target("", "").Return(&ClusterTarget{Endpoint: "https://c"}, nil),
		f.provisioner.EXPECT().Provision(ctx, "", "", rules).
			Return(&entities.ClusterCredential{Endpoint: "https://c", Token: "sa-token"}, nil),
		f.registrar.EXPECT().Register(ctx, "yc-prod", gomock.Any()).
			Return(&entities.ConnectedCluster{Name: "yc-prod", Connected: true}, nil),
	)
	f.provisioner.EXPECT().AccountName().Return("kube-system/paas-platform")

	cluster, err := f.uc.Execute(ctx, ConnectClusterInput{Name: "yc-prod"}, agree)

	require.NoError(t, err)
	assert.True(t, cluster.Connected)
}

// Владелец не согласился — в кластере ничего не меняется.
func TestConnectClusterRefused(t *testing.T) {
	f := newConnectFixture(t)
	ctx := context.Background()
	f.access.EXPECT().RequiredAccess(ctx).Return(rules, nil)
	f.provisioner.EXPECT().Target("", "").Return(&ClusterTarget{Endpoint: "https://c"}, nil)
	f.provisioner.EXPECT().AccountName().Return("kube-system/paas-platform")
	// Provision и Register не ожидаются.

	_, err := f.uc.Execute(ctx, ConnectClusterInput{Name: "yc-prod"}, refuse)

	assert.ErrorIs(t, err, entities.ErrCancelled)
}

// План показывается по тому, что отдала платформа: команда свой список не
// придумывает, иначе владелец подтвердил бы не то, что применится.
func TestConnectClusterShowsPlatformRules(t *testing.T) {
	f := newConnectFixture(t)
	ctx := context.Background()
	f.access.EXPECT().RequiredAccess(ctx).Return(rules, nil)
	f.provisioner.EXPECT().Target("", "").Return(&ClusterTarget{Endpoint: "https://c"}, nil)
	f.provisioner.EXPECT().AccountName().Return("kube-system/paas-platform")

	var shown ConnectClusterPlan
	_, _ = f.uc.Execute(ctx, ConnectClusterInput{Name: "yc-prod"}, func(p ConnectClusterPlan) (bool, error) {
		shown = p
		return false, nil
	})

	assert.Equal(t, rules, shown.Rules)
	assert.Equal(t, "https://c", shown.Endpoint)
	assert.Equal(t, "kube-system/paas-platform", shown.ServiceAccount)
}

// Кластер уже изменён, а платформа отказала — ошибка доносится как есть, чтобы
// владелец понял, что повтор безопасен (провижининг идемпотентен).
func TestConnectClusterRegistrationFails(t *testing.T) {
	f := newConnectFixture(t)
	ctx := context.Background()
	rejected := errors.New("платформа отклонила запрос: имя занято")
	f.access.EXPECT().RequiredAccess(ctx).Return(rules, nil)
	f.provisioner.EXPECT().Target("", "").Return(&ClusterTarget{Endpoint: "https://c"}, nil)
	f.provisioner.EXPECT().AccountName().Return("kube-system/paas-platform")
	f.provisioner.EXPECT().Provision(ctx, "", "", rules).Return(&entities.ClusterCredential{}, nil)
	f.registrar.EXPECT().Register(ctx, "yc-prod", gomock.Any()).Return(nil, rejected)

	_, err := f.uc.Execute(ctx, ConnectClusterInput{Name: "yc-prod"}, agree)

	assert.ErrorIs(t, err, rejected)
}
