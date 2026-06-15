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

func rootWith(checker CompatibilityChecker, out *bytes.Buffer) *cli.Command {
	return &cli.Command{
		Name:   "paas-cli",
		Writer: out,
		Commands: []*cli.Command{{
			Name:     "protocols",
			Commands: []*cli.Command{New(checker).CLICommand()},
		}},
	}
}

func run(t *testing.T, checker CompatibilityChecker, out *bytes.Buffer) error {
	t.Helper()
	return rootWith(checker, out).Run(context.Background(),
		[]string{"paas-cli", "protocols", "compatibility", "svc", "openapi.json"})
}

func TestCommandRun_Compatible(t *testing.T) {
	ctrl := gomock.NewController(t)
	checker := NewMockCompatibilityChecker(ctrl)
	checker.EXPECT().
		Execute(gomock.Any(), usecases.CheckCompatibilityInput{ServiceID: "svc", CandidatePath: "openapi.json"}).
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
	assert.Contains(t, out.String(), "несравнимо (снимок не разобран)")
}

func TestCommandRun_RequiresTwoArgs(t *testing.T) {
	ctrl := gomock.NewController(t)
	checker := NewMockCompatibilityChecker(ctrl)
	// Execute не вызывается — аргументы не прошли разбор.

	for _, extra := range [][]string{{}, {"svc"}, {"svc", "a", "b"}} {
		root := rootWith(checker, &bytes.Buffer{})
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
