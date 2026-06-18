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

func TestSyncProtocolsExecute_FetchesAllDependencies(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(&entities.Manifest{
		Service:      &entities.ManifestService{Name: "frontend"},
		Dependencies: []entities.ManifestDependency{{Name: "paas-backend"}, {Name: "billing"}},
	}, nil)

	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend", "billing"}).
		Return(map[string]string{"paas-backend": "id-backend", "billing": "id-billing"}, nil)

	backend := &entities.Protocol{ServiceName: "paas-backend", VersionNumber: 5, Document: []byte(validDoc)}
	billing := &entities.Protocol{ServiceName: "billing", VersionNumber: 2, Document: []byte(validDoc)}
	source.EXPECT().FetchProtocol(gomock.Any(), "id-backend").Return(backend, nil)
	source.EXPECT().FetchProtocol(gomock.Any(), "id-billing").Return(billing, nil)
	store.EXPECT().Save(gomock.Any(), backend, "protocols").Return("protocols/paas-backend/openapi.json", nil)
	store.EXPECT().Save(gomock.Any(), billing, "protocols").Return("protocols/billing/openapi.json", nil)

	got, err := NewSyncProtocols(manifests, resolver, source, store).
		Execute(context.Background(), SyncProtocolsInput{ManifestPath: "protocols.toml"})

	require.NoError(t, err)
	assert.Equal(t, "protocols", got.Destination)
	require.Len(t, got.Protocols, 2)
	assert.Equal(t, "paas-backend", got.Protocols[0].ServiceName)
	assert.Equal(t, "protocols/billing/openapi.json", got.Protocols[1].Path)
}

func TestSyncProtocolsExecute_PartialPerDependency(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	manifests.EXPECT().Read(gomock.Any(), gomock.Any()).Return(&entities.Manifest{
		Service:      &entities.ManifestService{Name: "frontend"},
		Dependencies: []entities.ManifestDependency{{Name: "billing", Methods: []string{"op-a"}}},
	}, nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"billing"}).
		Return(map[string]string{"billing": "id-billing"}, nil)
	source.EXPECT().FetchProtocol(gomock.Any(), "id-billing").
		Return(&entities.Protocol{ServiceName: "billing", Document: []byte(twoOpDoc)}, nil)
	store.EXPECT().Save(gomock.Any(), gomock.Any(), "protocols").
		DoAndReturn(func(_ context.Context, saved *entities.Protocol, _ string) (string, error) {
			assert.Contains(t, string(saved.Document), "op-a")
			assert.NotContains(t, string(saved.Document), "op-b")
			return "protocols/billing/openapi.json", nil
		})

	_, err := NewSyncProtocols(manifests, resolver, source, store).
		Execute(context.Background(), SyncProtocolsInput{ManifestPath: "protocols.toml"})
	require.NoError(t, err)
}

func TestSyncProtocolsExecute_DestinationFromManifest(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	manifests.EXPECT().Read(gomock.Any(), gomock.Any()).Return(&entities.Manifest{
		Service:      &entities.ManifestService{Name: "frontend"},
		Destination:  "vendor/api",
		Dependencies: []entities.ManifestDependency{{Name: "billing"}},
	}, nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"billing"}).
		Return(map[string]string{"billing": "id-billing"}, nil)
	source.EXPECT().FetchProtocol(gomock.Any(), "id-billing").
		Return(&entities.Protocol{ServiceName: "billing", Document: []byte(validDoc)}, nil)
	store.EXPECT().Save(gomock.Any(), gomock.Any(), "vendor/api").Return("vendor/api/billing/openapi.json", nil)

	got, err := NewSyncProtocols(manifests, resolver, source, store).
		Execute(context.Background(), SyncProtocolsInput{ManifestPath: "protocols.toml"})
	require.NoError(t, err)
	assert.Equal(t, "vendor/api", got.Destination)
}

func TestSyncProtocolsExecute_OverrideBeatsManifest(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	manifests.EXPECT().Read(gomock.Any(), gomock.Any()).Return(&entities.Manifest{
		Service:      &entities.ManifestService{Name: "frontend"},
		Destination:  "vendor/api",
		Dependencies: []entities.ManifestDependency{{Name: "billing"}},
	}, nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"billing"}).
		Return(map[string]string{"billing": "id-billing"}, nil)
	source.EXPECT().FetchProtocol(gomock.Any(), "id-billing").
		Return(&entities.Protocol{ServiceName: "billing", Document: []byte(validDoc)}, nil)
	store.EXPECT().Save(gomock.Any(), gomock.Any(), "flag-dir").Return("flag-dir/billing/openapi.json", nil)

	got, err := NewSyncProtocols(manifests, resolver, source, store).
		Execute(context.Background(), SyncProtocolsInput{ManifestPath: "protocols.toml", DestinationOverride: "flag-dir"})
	require.NoError(t, err)
	assert.Equal(t, "flag-dir", got.Destination)
}

func TestSyncProtocolsExecute_InvalidManifest_NoFetch(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	manifests.EXPECT().Read(gomock.Any(), gomock.Any()).Return(&entities.Manifest{}, nil)
	// resolver/source/store не вызываются — пустой манифест не молчит.

	_, err := NewSyncProtocols(manifests, resolver, source, store).
		Execute(context.Background(), SyncProtocolsInput{ManifestPath: "protocols.toml"})
	assert.ErrorIs(t, err, entities.ErrManifestNoService)
}

func TestSyncProtocolsExecute_UnknownService_Aborts(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	manifests.EXPECT().Read(gomock.Any(), gomock.Any()).Return(&entities.Manifest{
		Service:      &entities.ManifestService{Name: "frontend"},
		Dependencies: []entities.ManifestDependency{{Name: "ghost"}, {Name: "billing"}},
	}, nil)
	// платформа не вернула "ghost" — его нет в карте; прогон валится на первой
	// зависимости, контракты не тянутся.
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"ghost", "billing"}).
		Return(map[string]string{"billing": "id-billing"}, nil)

	_, err := NewSyncProtocols(manifests, resolver, source, store).
		Execute(context.Background(), SyncProtocolsInput{ManifestPath: "protocols.toml"})
	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
	assert.Contains(t, err.Error(), "ghost")
}

func TestSyncProtocolsExecute_ReadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	readErr := errors.New("no such file")
	manifests.EXPECT().Read(gomock.Any(), gomock.Any()).Return(nil, readErr)

	_, err := NewSyncProtocols(manifests, NewMockServiceResolver(ctrl), NewMockProtocolSource(ctrl), NewMockProtocolStore(ctrl)).
		Execute(context.Background(), SyncProtocolsInput{ManifestPath: "protocols.toml"})
	assert.ErrorIs(t, err, readErr)
}
