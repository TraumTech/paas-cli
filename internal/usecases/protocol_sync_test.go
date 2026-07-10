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
	source.EXPECT().FetchProtocol(gomock.Any(), "id-backend", gomock.Nil()).Return(backend, false, nil)
	source.EXPECT().FetchProtocol(gomock.Any(), "id-billing", gomock.Nil()).Return(billing, false, nil)
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
	// Сужение выполняет платформа: methods зависимости уходят в запрос, к себе
	// приходит и кладётся уже частичный контракт (CLI-09).
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	manifests.EXPECT().Read(gomock.Any(), gomock.Any()).Return(&entities.Manifest{
		Service:      &entities.ManifestService{Name: "frontend"},
		Dependencies: []entities.ManifestDependency{{Name: "billing", Methods: []string{"GET /a"}}},
	}, nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"billing"}).
		Return(map[string]string{"billing": "id-billing"}, nil)
	partial := &entities.Protocol{ServiceName: "billing", Document: []byte(partialDoc)}
	source.EXPECT().FetchProtocol(gomock.Any(), "id-billing", []string{"GET /a"}).
		Return(partial, false, nil)
	store.EXPECT().Save(gomock.Any(), partial, "protocols").Return("protocols/billing/openapi.json", nil)

	got, err := NewSyncProtocols(manifests, resolver, source, store).
		Execute(context.Background(), SyncProtocolsInput{ManifestPath: "protocols.toml"})
	require.NoError(t, err)
	require.Len(t, got.Protocols, 1)
	assert.False(t, got.Protocols[0].NarrowingSkipped)
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
	source.EXPECT().FetchProtocol(gomock.Any(), "id-billing", gomock.Nil()).
		Return(&entities.Protocol{ServiceName: "billing", Document: []byte(validDoc)}, false, nil)
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
	source.EXPECT().FetchProtocol(gomock.Any(), "id-billing", gomock.Nil()).
		Return(&entities.Protocol{ServiceName: "billing", Document: []byte(validDoc)}, false, nil)
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

// Смешанный манифест: OpenAPI-зависимость кладётся как прежде, gRPC — в родном
// виде (.proto); в результатах виден формат каждого контракта.
func TestSyncProtocolsExecute_MixedFormats(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	manifests.EXPECT().Read(gomock.Any(), gomock.Any()).Return(&entities.Manifest{
		Service:      &entities.ManifestService{Name: "paas-backend"},
		Dependencies: []entities.ManifestDependency{{Name: "billing"}, {Name: "paas-protocols"}},
	}, nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"billing", "paas-protocols"}).
		Return(map[string]string{"billing": "id-billing", "paas-protocols": "id-registry"}, nil)

	openapi := &entities.Protocol{ServiceName: "billing", VersionNumber: 2, Format: entities.ProtocolFormatOpenAPI, Document: []byte(validDoc)}
	grpc := &entities.Protocol{ServiceName: "paas-protocols", VersionNumber: 1, Format: entities.ProtocolFormatGRPC, Document: []byte("syntax = \"proto3\";")}
	source.EXPECT().FetchProtocol(gomock.Any(), "id-billing", gomock.Nil()).Return(openapi, false, nil)
	source.EXPECT().FetchProtocol(gomock.Any(), "id-registry", gomock.Nil()).Return(grpc, false, nil)
	store.EXPECT().Save(gomock.Any(), openapi, "protocols").Return("protocols/billing/openapi.json", nil)
	store.EXPECT().Save(gomock.Any(), grpc, "protocols").Return("protocols/paas-protocols/contract.proto", nil)

	got, err := NewSyncProtocols(manifests, resolver, source, store).
		Execute(context.Background(), SyncProtocolsInput{ManifestPath: "protocols.toml"})

	require.NoError(t, err)
	require.Len(t, got.Protocols, 2)
	assert.Equal(t, entities.ProtocolFormatOpenAPI, got.Protocols[0].Format)
	assert.Equal(t, entities.ProtocolFormatGRPC, got.Protocols[1].Format)
	assert.Equal(t, "protocols/paas-protocols/contract.proto", got.Protocols[1].Path)
}

// methods у gRPC-зависимости объявляют используемые методы (CLI-20): sync не
// падает — платформа приносит контракт целиком с narrowingSkipped, и это видно
// в результате прогона (уточнение CLI-19).
func TestSyncProtocolsExecute_GRPCMethodsBringFullContract(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	manifests.EXPECT().Read(gomock.Any(), gomock.Any()).Return(&entities.Manifest{
		Service:      &entities.ManifestService{Name: "paas-backend"},
		Dependencies: []entities.ManifestDependency{{Name: "paas-protocols", Methods: []string{"traumtech.paas_protocols.v1.RegistryService/PublishProtocol"}}},
	}, nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-protocols"}).
		Return(map[string]string{"paas-protocols": "id-registry"}, nil)
	full := &entities.Protocol{ServiceName: "paas-protocols", Format: entities.ProtocolFormatGRPC, Document: []byte("syntax = \"proto3\";")}
	source.EXPECT().FetchProtocol(gomock.Any(), "id-registry", []string{"traumtech.paas_protocols.v1.RegistryService/PublishProtocol"}).
		Return(full, true, nil)
	// Кладётся именно полный контракт — платформа честно сообщила, что сужение
	// для формата не поддерживается.
	store.EXPECT().Save(gomock.Any(), full, "protocols").Return("protocols/paas-protocols/contract.proto", nil)

	got, err := NewSyncProtocols(manifests, resolver, source, store).
		Execute(context.Background(), SyncProtocolsInput{ManifestPath: "protocols.toml"})

	require.NoError(t, err)
	require.Len(t, got.Protocols, 1)
	assert.True(t, got.Protocols[0].NarrowingSkipped, "в отчёте видно, что сужение не выполнено")
}

// Пустой gRPC-ответ платформы не доходит до раскладки — валидация отсекает до Save.
func TestSyncProtocolsExecute_EmptyGRPCDocument_NoSave(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	manifests.EXPECT().Read(gomock.Any(), gomock.Any()).Return(&entities.Manifest{
		Service:      &entities.ManifestService{Name: "paas-backend"},
		Dependencies: []entities.ManifestDependency{{Name: "paas-protocols"}},
	}, nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-protocols"}).
		Return(map[string]string{"paas-protocols": "id-registry"}, nil)
	source.EXPECT().FetchProtocol(gomock.Any(), "id-registry", gomock.Nil()).
		Return(&entities.Protocol{ServiceName: "paas-protocols", Format: entities.ProtocolFormatGRPC, Document: []byte("  ")}, false, nil)

	_, err := NewSyncProtocols(manifests, resolver, source, store).
		Execute(context.Background(), SyncProtocolsInput{ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, entities.ErrEmptyProtocol)
}
