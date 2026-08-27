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

// gRPC-кандидат (CLI-21): формат доходит до платформы, .proto не гоняется через
// JSON-проверку OpenAPI; пустой .proto — честный отказ до платформы.
func TestCheckCompatibilityExecute_GRPCCandidate(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	source := NewMockCompatibilitySource(ctrl)

	proto := []byte("syntax = \"proto3\";\npackage traumtech.paas_protocols.v1;")
	report := &entities.CompatibilityReport{Breaking: false}
	reader.EXPECT().Read(gomock.Any(), "registry.proto").Return(proto, nil)
	source.EXPECT().CheckCompatibility(gomock.Any(), "svc", "", entities.ProtocolFormatGRPC, proto).Return(report, nil)

	got, err := NewCheckCompatibility(reader, source).Execute(context.Background(),
		CheckCompatibilityInput{ServiceID: "svc", Format: entities.ProtocolFormatGRPC, CandidatePath: "registry.proto"})

	require.NoError(t, err)
	assert.Same(t, report, got)
}

func TestCheckCompatibilityExecute_EmptyGRPCCandidate(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	source := NewMockCompatibilitySource(ctrl)

	reader.EXPECT().Read(gomock.Any(), "registry.proto").Return([]byte("  \n"), nil)
	// платформа не вызывается.

	_, err := NewCheckCompatibility(reader, source).Execute(context.Background(),
		CheckCompatibilityInput{ServiceID: "svc", Format: entities.ProtocolFormatGRPC, CandidatePath: "registry.proto"})

	assert.ErrorIs(t, err, entities.ErrEmptyProtocol)
}

func TestCheckCompatibilityExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	source := NewMockCompatibilitySource(ctrl)

	report := &entities.CompatibilityReport{Breaking: true}
	reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	source.EXPECT().CheckCompatibility(gomock.Any(), "svc", "", entities.ProtocolFormatOpenAPI, []byte(validDoc)).Return(report, nil)

	got, err := NewCheckCompatibility(reader, source).Execute(context.Background(),
		CheckCompatibilityInput{ServiceID: "svc", Format: entities.ProtocolFormatOpenAPI, CandidatePath: "openapi.json"})

	require.NoError(t, err)
	assert.Same(t, report, got)
}

func TestCheckCompatibilityExecute_ReadError_NoCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	source := NewMockCompatibilitySource(ctrl)

	readErr := errors.New("no such file")
	reader.EXPECT().Read(gomock.Any(), "missing.json").Return(nil, readErr)
	// CheckCompatibility не вызывается — платформу не дёргаем без документа.

	_, err := NewCheckCompatibility(reader, source).Execute(context.Background(),
		CheckCompatibilityInput{ServiceID: "svc", CandidatePath: "missing.json"})

	assert.ErrorIs(t, err, readErr)
}

func TestCheckCompatibilityExecute_InvalidCandidate_NoCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	source := NewMockCompatibilitySource(ctrl)

	reader.EXPECT().Read(gomock.Any(), "bad.json").Return([]byte("<html>"), nil)
	// невалидный кандидат на платформу не уходит.

	_, err := NewCheckCompatibility(reader, source).Execute(context.Background(),
		CheckCompatibilityInput{ServiceID: "svc", CandidatePath: "bad.json"})

	assert.ErrorIs(t, err, entities.ErrInvalidProtocol)
}

func TestCheckCompatibilityExecute_SourceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	source := NewMockCompatibilitySource(ctrl)

	reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	source.EXPECT().CheckCompatibility(gomock.Any(), "svc", "", entities.ProtocolFormatOpenAPI, []byte(validDoc)).Return(nil, entities.ErrServiceNotFound)

	_, err := NewCheckCompatibility(reader, source).Execute(context.Background(),
		CheckCompatibilityInput{ServiceID: "svc", Format: entities.ProtocolFormatOpenAPI, CandidatePath: "openapi.json"})

	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}

// Манифестный режим (CLI-23): каждая запись перечня сверяется под своим именем,
// итог несёт имена; исчезнувший из манифеста протокол без потребителей —
// предупреждение.
func TestCheckManifestCompatibilityExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	source := NewMockCompatibilitySource(ctrl)
	registry := NewMockRegistryDirectory(ctrl)

	proto := []byte("syntax = \"proto3\";")
	manifest := &entities.Manifest{
		Service: &entities.ManifestService{Name: "paas-backend"},
		Protocols: []entities.ManifestProtocol{
			{Name: "http", Contract: "openapi.json"},
			{Name: "internal-grpc", Contract: "edge.proto", Format: "grpc"},
		},
	}
	manifests.EXPECT().Read(gomock.Any(), "paas.toml").Return(manifest, nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	reader.EXPECT().Read(gomock.Any(), "edge.proto").Return(proto, nil)
	registry.EXPECT().ListProtocols(gomock.Any(), "svc").Return([]entities.RegistryProtocol{
		{Name: "http", Format: "openapi"},
		{Name: "admin", Format: "openapi"},
	}, nil)
	source.EXPECT().CheckCompatibility(gomock.Any(), "svc", "http", entities.ProtocolFormatOpenAPI, []byte(validDoc)).
		Return(&entities.CompatibilityReport{Breaking: true}, nil)
	source.EXPECT().CheckCompatibility(gomock.Any(), "svc", "internal-grpc", entities.ProtocolFormatGRPC, proto).
		Return(&entities.CompatibilityReport{}, nil)

	got, err := NewCheckManifestCompatibility(manifests, resolver, reader, source, registry).Execute(context.Background(),
		CheckManifestCompatibilityInput{ManifestPath: "paas.toml"})

	require.NoError(t, err)
	require.Len(t, got.Reports, 2)
	assert.Equal(t, "http", got.Reports[0].Name)
	assert.True(t, got.Reports[0].Report.Breaking)
	assert.Equal(t, "internal-grpc", got.Reports[1].Name)
	assert.Equal(t, []string{"admin"}, got.Orphaned)
}

// Протокол по умолчанию исчез из манифеста, а от него зависят потребители —
// проверка отказывает до единственной сверки: тот же гейт, что у публикации.
func TestCheckManifestCompatibilityExecute_OrphanedWithConsumers(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	source := NewMockCompatibilitySource(ctrl)
	registry := NewMockRegistryDirectory(ctrl)

	manifest := &entities.Manifest{
		Service:   &entities.ManifestService{Name: "paas-backend"},
		Protocols: []entities.ManifestProtocol{{Name: "http", Contract: "openapi.json"}},
	}
	manifests.EXPECT().Read(gomock.Any(), "paas.toml").Return(manifest, nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	registry.EXPECT().ListProtocols(gomock.Any(), "svc").Return([]entities.RegistryProtocol{
		{Name: entities.DefaultProtocolName, Format: "openapi"},
	}, nil)
	registry.EXPECT().ListConsumers(gomock.Any(), "svc").Return([]entities.RegisteredConsumer{
		{ServiceName: "paas-frontend", VersionNumber: 12},
	}, nil)
	// сверок нет — гейт отказал раньше.

	_, err := NewCheckManifestCompatibility(manifests, resolver, reader, source, registry).Execute(context.Background(),
		CheckManifestCompatibilityInput{ManifestPath: "paas.toml"})

	var orphaned *entities.OrphanedProtocolError
	require.ErrorAs(t, err, &orphaned)
	assert.Equal(t, entities.DefaultProtocolName, orphaned.Name)
}
