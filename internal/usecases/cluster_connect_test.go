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
	directory   *MockClusterDirectory
	uc          *ConnectClusterUseCase
}

func newConnectFixture(t *testing.T) *connectFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := &connectFixture{
		access:      NewMockClusterAccessSource(ctrl),
		provisioner: NewMockClusterProvisioner(ctrl),
		registrar:   NewMockClusterRegistrar(ctrl),
		directory:   NewMockClusterDirectory(ctrl),
	}
	f.uc = NewConnectCluster(f.access, f.provisioner, f.registrar, f.directory)
	return f
}

// expectFresh — обычное начало: права от платформы, координаты кластера с
// заданными зонами, кластера с таким именем ещё нет.
func (f *connectFixture) expectFresh(ctx context.Context, zones ...string) {
	f.access.EXPECT().RequiredAccess(ctx).Return(rules, nil)
	f.provisioner.EXPECT().Target("", "").Return(&ClusterTarget{Endpoint: "https://c", Zones: zones}, nil)
	f.directory.EXPECT().ListClusters(ctx).Return(nil, nil)
	f.provisioner.EXPECT().AccountName().Return("kube-system/paas-platform").AnyTimes()
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
		f.provisioner.EXPECT().Target("", "").Return(&ClusterTarget{Endpoint: "https://c", Zones: []string{"a", "b"}}, nil),
		f.directory.EXPECT().ListClusters(ctx).Return(nil, nil),
		f.provisioner.EXPECT().Provision(ctx, "", "", rules).
			Return(&entities.ClusterCredential{Endpoint: "https://c", Token: "sa-token"}, nil),
		// Топология предложена по зонам: две зоны — региональный.
		f.registrar.EXPECT().Register(ctx, "yc-prod", gomock.Any(), entities.ClusterTopologyRegional).
			Return(&entities.ConnectedCluster{Name: "yc-prod", Connected: true, Topology: entities.ClusterTopologyRegional}, nil),
	)
	f.provisioner.EXPECT().AccountName().Return("kube-system/paas-platform")

	cluster, existing, err := f.uc.Execute(ctx, ConnectClusterInput{Name: "yc-prod"}, agree)

	require.NoError(t, err)
	assert.Nil(t, existing)
	assert.True(t, cluster.Connected)
}

// Топология — заявление владельца: заданная явно перебивает предложенную, а
// расхождение с нодами лишь показывается.
func TestConnectClusterTopology(t *testing.T) {
	tests := []struct {
		name         string
		zones        []string
		declared     entities.ClusterTopology
		want         entities.ClusterTopology
		wantDeclared bool
		wantWarning  bool
	}{
		{name: "одна зона — предлагается зональный", zones: []string{"a"}, want: entities.ClusterTopologyZonal},
		{name: "без меток — предлагается зональный", want: entities.ClusterTopologyZonal},
		{name: "две зоны — предлагается региональный", zones: []string{"a", "b"}, want: entities.ClusterTopologyRegional},
		{name: "владелец задал зональный при двух зонах", zones: []string{"a", "b"}, declared: entities.ClusterTopologyZonal, want: entities.ClusterTopologyZonal, wantDeclared: true},
		{name: "владелец обещает региональный при одной зоне", zones: []string{"a"}, declared: entities.ClusterTopologyRegional, want: entities.ClusterTopologyRegional, wantDeclared: true, wantWarning: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newConnectFixture(t)
			ctx := context.Background()
			f.expectFresh(ctx, tt.zones...)
			f.provisioner.EXPECT().Provision(ctx, "", "", rules).Return(&entities.ClusterCredential{}, nil)
			f.registrar.EXPECT().Register(ctx, "yc-prod", gomock.Any(), tt.want).
				Return(&entities.ConnectedCluster{Name: "yc-prod", Topology: tt.want}, nil)

			var shown ConnectClusterPlan
			_, _, err := f.uc.Execute(ctx, ConnectClusterInput{Name: "yc-prod", Topology: tt.declared},
				func(p ConnectClusterPlan) (bool, error) {
					shown = p
					return true, nil
				})

			require.NoError(t, err)
			assert.Equal(t, tt.want, shown.Topology)
			assert.Equal(t, tt.wantDeclared, shown.TopologyDeclared)
			assert.Equal(t, tt.zones, shown.Zones)
			assert.Equal(t, tt.wantWarning, len(shown.Warnings) > 0)
		})
	}
}

// Незнакомая топология отсекается до обращения к платформе и кластеру.
func TestConnectClusterRejectsUnknownTopology(t *testing.T) {
	f := newConnectFixture(t)

	_, _, err := f.uc.Execute(context.Background(), ConnectClusterInput{Name: "yc-prod", Topology: "global"}, agree)

	assert.ErrorIs(t, err, entities.ErrUnknownClusterTopology)
}

// Повтор для уже подключённого кластера ничего не меняет: ни в кластере, ни
// на платформе; заявленная топология остаётся, расхождение с нодами видно.
func TestConnectClusterAlreadyConnected(t *testing.T) {
	f := newConnectFixture(t)
	ctx := context.Background()
	f.access.EXPECT().RequiredAccess(ctx).Return(rules, nil)
	f.provisioner.EXPECT().Target("", "").Return(&ClusterTarget{Endpoint: "https://c", Zones: []string{"a"}}, nil)
	f.directory.EXPECT().ListClusters(ctx).Return([]entities.ConnectedCluster{
		{Name: "other"},
		{Name: "yc-prod", Endpoint: "https://c", Topology: entities.ClusterTopologyRegional},
	}, nil)
	// Provision и Register не ожидаются, подтверждение не спрашивается.

	cluster, existing, err := f.uc.Execute(ctx, ConnectClusterInput{Name: "yc-prod"}, refuse)

	require.NoError(t, err)
	assert.Nil(t, cluster)
	require.NotNil(t, existing)
	assert.Equal(t, entities.ClusterTopologyRegional, existing.Cluster.Topology)
	assert.Equal(t, entities.ClusterTopologyZonal, existing.Proposed)
	assert.Equal(t, []string{"a"}, existing.Zones)
}

// Владелец не согласился — в кластере ничего не меняется.
func TestConnectClusterRefused(t *testing.T) {
	f := newConnectFixture(t)
	ctx := context.Background()
	f.expectFresh(ctx)
	// Provision и Register не ожидаются.

	_, _, err := f.uc.Execute(ctx, ConnectClusterInput{Name: "yc-prod"}, refuse)

	assert.ErrorIs(t, err, entities.ErrCancelled)
}

// План показывается по тому, что отдала платформа: команда свой список не
// придумывает, иначе владелец подтвердил бы не то, что применится.
func TestConnectClusterShowsPlatformRules(t *testing.T) {
	f := newConnectFixture(t)
	ctx := context.Background()
	f.expectFresh(ctx)

	var shown ConnectClusterPlan
	_, _, _ = f.uc.Execute(ctx, ConnectClusterInput{Name: "yc-prod"}, func(p ConnectClusterPlan) (bool, error) {
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
	f.expectFresh(ctx)
	f.provisioner.EXPECT().Provision(ctx, "", "", rules).Return(&entities.ClusterCredential{}, nil)
	f.registrar.EXPECT().Register(ctx, "yc-prod", gomock.Any(), entities.ClusterTopologyZonal).Return(nil, rejected)

	_, _, err := f.uc.Execute(ctx, ConnectClusterInput{Name: "yc-prod"}, agree)

	assert.ErrorIs(t, err, rejected)
}
