package protocolcompatibilitycommandcli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

func rootWith(checker CompatibilityChecker, manifest ManifestCompatibilityChecker, out *bytes.Buffer) *cli.Command {
	return &cli.Command{
		Name:   "paas-cli",
		Writer: out,
		Commands: []*cli.Command{{
			Name:     "protocols",
			Commands: []*cli.Command{New(checker, manifest).CLICommand()},
		}},
	}
}

func run(t *testing.T, checker CompatibilityChecker, out *bytes.Buffer) error {
	t.Helper()
	return rootWith(checker, nil, out).Run(context.Background(),
		[]string{"paas-cli", "protocols", "compatibility", "svc", "openapi.json"})
}

func TestCommandRun_Compatible(t *testing.T) {
	ctrl := gomock.NewController(t)
	checker := NewMockCompatibilityChecker(ctrl)
	checker.EXPECT().
		Execute(gomock.Any(), usecases.CheckCompatibilityInput{ServiceID: "svc", Format: entities.ProtocolFormatOpenAPI, CandidatePath: "openapi.json"}).
		Return(&entities.CompatibilityReport{Breaking: false, Consumers: []entities.ConsumerCompatibility{
			{ServiceName: "frontend", VersionNumber: 5, Comparable: true, Changes: []entities.CompatibilityChange{
				{Kind: "operation-added", Operation: "GET /y", Description: "новый эндпоинт"},
			}},
		}}, nil)

	var out bytes.Buffer
	require.NoError(t, run(t, checker, &out))
	assert.Contains(t, out.String(), "frontend v5: совместимо")
	assert.Contains(t, out.String(), "[compatible] operation-added GET /y — новый эндпоинт")
}

// PRT-27: изменение отпущенного атрибута видно в отчёте, но помечено отказом и
// потребителя не ломает.
func TestCommandRun_WaivedAttribute(t *testing.T) {
	ctrl := gomock.NewController(t)
	checker := NewMockCompatibilityChecker(ctrl)
	checker.EXPECT().Execute(gomock.Any(), gomock.Any()).
		Return(&entities.CompatibilityReport{Breaking: false, Consumers: []entities.ConsumerCompatibility{
			{ServiceName: "frontend", VersionNumber: 5, Comparable: true, Changes: []entities.CompatibilityChange{
				{Kind: "removed", Operation: "shop.v1.Svc/Get", Description: "поле shop.v1.Order.note (№5) удалено",
					Attribute: "shop.v1.Order.note", Waived: true},
			}},
		}}, nil)

	var out bytes.Buffer
	require.NoError(t, run(t, checker, &out))
	assert.Contains(t, out.String(), "frontend v5: совместимо")
	assert.Contains(t, out.String(), "[отказ] removed shop.v1.Svc/Get — поле shop.v1.Order.note (№5) удалено")
}

func TestCommandRun_NoConsumers(t *testing.T) {
	ctrl := gomock.NewController(t)
	checker := NewMockCompatibilityChecker(ctrl)
	checker.EXPECT().Execute(gomock.Any(), gomock.Any()).
		Return(&entities.CompatibilityReport{}, nil)

	var out bytes.Buffer
	require.NoError(t, run(t, checker, &out))
	assert.Contains(t, out.String(), "никого не затрагивает")
}

func TestCommandRun_BreakingReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	checker := NewMockCompatibilityChecker(ctrl)
	checker.EXPECT().Execute(gomock.Any(), gomock.Any()).
		Return(&entities.CompatibilityReport{Breaking: true, Consumers: []entities.ConsumerCompatibility{
			{ServiceName: "frontend", VersionNumber: 5, Comparable: true, Breaking: true, Changes: []entities.CompatibilityChange{
				{Breaking: true, Kind: "operation-removed", Operation: "GET /x", Description: "удалён эндпоинт"},
			}},
		}}, nil)

	var out bytes.Buffer
	err := run(t, checker, &out)
	require.Error(t, err)
	assert.Contains(t, out.String(), "frontend v5: ЛОМАЕТ")
	assert.Contains(t, out.String(), "[BREAKING] operation-removed GET /x — удалён эндпоинт")
}

func TestCommandRun_Incomparable(t *testing.T) {
	ctrl := gomock.NewController(t)
	checker := NewMockCompatibilityChecker(ctrl)
	checker.EXPECT().Execute(gomock.Any(), gomock.Any()).
		Return(&entities.CompatibilityReport{Consumers: []entities.ConsumerCompatibility{
			{ServiceName: "legacy", VersionNumber: 1, Comparable: false},
		}}, nil)

	var out bytes.Buffer
	require.NoError(t, run(t, checker, &out))
	assert.Contains(t, out.String(), "не сверялось (снимок несравним с контрактом)")
}

func TestCommandRun_RequiresTwoArgs(t *testing.T) {
	ctrl := gomock.NewController(t)
	checker := NewMockCompatibilityChecker(ctrl)
	// Execute не вызывается — аргументы не прошли разбор; без аргументов —
	// манифестный режим, поэтому ошибочны только 1 и 3+.

	for _, extra := range [][]string{{"svc"}, {"svc", "a", "b"}} {
		root := rootWith(checker, nil, &bytes.Buffer{})
		args := append([]string{"paas-cli", "protocols", "compatibility"}, extra...)
		assert.Error(t, root.Run(context.Background(), args))
	}
}

func TestCommandRun_PropagatesUseCaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	checker := NewMockCompatibilityChecker(ctrl)
	checker.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, entities.ErrServiceNotFound)

	err := run(t, checker, &bytes.Buffer{})
	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}

// Опечатка в формате — ошибка до чтения файла и похода на платформу.
func TestCommandRun_UnsupportedFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	checker := NewMockCompatibilityChecker(ctrl)
	checker.EXPECT().Execute(gomock.Any(), gomock.Any()).Times(0)

	var out bytes.Buffer
	err := rootWith(checker, nil, &out).Run(context.Background(),
		[]string{"paas-cli", "protocols", "compatibility", "--format", "graphql", "svc", "x.proto"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "graphql")
}

// Имя протокола-кандидата из флага доходит до use case (CLI-23).
func TestCommandRun_NameFlag(t *testing.T) {
	ctrl := gomock.NewController(t)
	checker := NewMockCompatibilityChecker(ctrl)
	checker.EXPECT().
		Execute(gomock.Any(), usecases.CheckCompatibilityInput{ServiceID: "svc", Name: "admin", Format: entities.ProtocolFormatOpenAPI, CandidatePath: "admin.json"}).
		Return(&entities.CompatibilityReport{}, nil)

	var out bytes.Buffer
	err := rootWith(checker, nil, &out).Run(context.Background(),
		[]string{"paas-cli", "protocols", "compatibility", "--name", "admin", "svc", "admin.json"})

	require.NoError(t, err)
}

func runManifest(t *testing.T, manifest ManifestCompatibilityChecker, out *bytes.Buffer) error {
	t.Helper()
	return rootWith(nil, manifest, out).Run(context.Background(),
		[]string{"paas-cli", "protocols", "compatibility"})
}

// Без аргументов — манифестный режим (CLI-23): сводка по каждому протоколу
// подписана именем, у безымянной записи вид прежний.
func TestCommandRun_ManifestMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifest := NewMockManifestCompatibilityChecker(ctrl)
	manifest.EXPECT().
		Execute(gomock.Any(), usecases.CheckManifestCompatibilityInput{ManifestPath: ""}).
		Return(&entities.ManifestCompatibilityReport{Reports: []entities.NamedCompatibilityReport{
			{Name: "http", Report: entities.CompatibilityReport{Consumers: []entities.ConsumerCompatibility{
				{ServiceName: "frontend", VersionNumber: 5, Comparable: true},
			}}},
			{Name: "internal-grpc", Report: entities.CompatibilityReport{}},
		}}, nil)

	var out bytes.Buffer
	require.NoError(t, runManifest(t, manifest, &out))
	assert.Contains(t, out.String(), `Протокол "http":`)
	assert.Contains(t, out.String(), "frontend v5: совместимо, без изменений")
	assert.Contains(t, out.String(), `Протокол "internal-grpc":`)
	assert.Contains(t, out.String(), "никого не затрагивает")
}

// Ломающий кандидат любого протокола — ненулевой код выхода; сводки по
// остальным протоколам всё равно печатаются.
func TestCommandRun_ManifestModeBreaking(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifest := NewMockManifestCompatibilityChecker(ctrl)
	manifest.EXPECT().Execute(gomock.Any(), gomock.Any()).
		Return(&entities.ManifestCompatibilityReport{Reports: []entities.NamedCompatibilityReport{
			{Name: "http", Report: entities.CompatibilityReport{Breaking: true, Consumers: []entities.ConsumerCompatibility{
				{ServiceName: "frontend", VersionNumber: 5, Comparable: true, Breaking: true},
			}}},
			{Name: "internal-grpc", Report: entities.CompatibilityReport{}},
		}}, nil)

	var out bytes.Buffer
	err := runManifest(t, manifest, &out)
	require.Error(t, err)
	assert.Contains(t, out.String(), "frontend v5: ЛОМАЕТ")
	assert.Contains(t, out.String(), `Протокол "internal-grpc":`)
}

// Осиротевший протокол без потребителей проверку не держит, но о нём
// предупреждается.
func TestCommandRun_ManifestModeOrphanedWarning(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifest := NewMockManifestCompatibilityChecker(ctrl)
	manifest.EXPECT().Execute(gomock.Any(), gomock.Any()).
		Return(&entities.ManifestCompatibilityReport{
			Reports:  []entities.NamedCompatibilityReport{{Name: "http", Report: entities.CompatibilityReport{}}},
			Orphaned: []string{"admin"},
		}, nil)

	var out bytes.Buffer
	require.NoError(t, runManifest(t, manifest, &out))
	assert.Contains(t, out.String(), `протокол "admin" остался в реестре, но исчез из манифеста`)
}

// gRPC-кандидат: формат из флага доходит до use case.
func TestCommandRun_GRPCFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	checker := NewMockCompatibilityChecker(ctrl)
	checker.EXPECT().
		Execute(gomock.Any(), usecases.CheckCompatibilityInput{ServiceID: "svc", Format: entities.ProtocolFormatGRPC, CandidatePath: "registry.proto"}).
		Return(&entities.CompatibilityReport{}, nil)

	var out bytes.Buffer
	err := rootWith(checker, nil, &out).Run(context.Background(),
		[]string{"paas-cli", "protocols", "compatibility", "--format", "grpc", "svc", "registry.proto"})

	require.NoError(t, err)
}
