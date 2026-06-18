package protocolpublishcommandcli

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

func rootWith(publisher ProtocolPublisher, out *bytes.Buffer) *cli.Command {
	return &cli.Command{
		Name:   "paas-cli",
		Writer: out,
		Commands: []*cli.Command{{
			Name:     "protocols",
			Commands: []*cli.Command{New(publisher).CLICommand()},
		}},
	}
}

func run(t *testing.T, publisher ProtocolPublisher, out *bytes.Buffer) error {
	t.Helper()
	return rootWith(publisher, out).Run(context.Background(),
		[]string{"paas-cli", "protocols", "publish", "ver"})
}

func TestCommandRun_PublishesAndShowsCompatibility(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := NewMockProtocolPublisher(ctrl)
	publisher.EXPECT().
		Execute(gomock.Any(), usecases.PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"}).
		Return(&entities.ProtocolPublication{VersionNumber: 7, Breaking: false, Consumers: []entities.ConsumerCompatibility{
			{ServiceName: "frontend", VersionNumber: 5, Comparable: true, Changes: []entities.CompatibilityChange{
				{Kind: "operation-added", Operation: "GET /y", Description: "новый эндпоинт"},
			}},
		}}, nil)

	var out bytes.Buffer
	require.NoError(t, run(t, publisher, &out))
	assert.Contains(t, out.String(), "опубликован под версией v7")
	assert.Contains(t, out.String(), "frontend v5: совместимо")
	assert.Contains(t, out.String(), "[compatible] operation-added GET /y — новый эндпоинт")
}

func TestCommandRun_NoConsumers(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := NewMockProtocolPublisher(ctrl)
	publisher.EXPECT().Execute(gomock.Any(), gomock.Any()).
		Return(&entities.ProtocolPublication{VersionNumber: 7}, nil)

	var out bytes.Buffer
	require.NoError(t, run(t, publisher, &out))
	assert.Contains(t, out.String(), "никого не затрагивает")
}

// Ломающее изменение только информирует: команда успешна (код 0), сводка
// предупреждает. Гейт ломающих изменений — отдельная проверка совместимости.
func TestCommandRun_BreakingStillSucceeds(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := NewMockProtocolPublisher(ctrl)
	publisher.EXPECT().Execute(gomock.Any(), gomock.Any()).
		Return(&entities.ProtocolPublication{VersionNumber: 8, Breaking: true, Consumers: []entities.ConsumerCompatibility{
			{ServiceName: "frontend", VersionNumber: 5, Comparable: true, Breaking: true, Changes: []entities.CompatibilityChange{
				{Breaking: true, Kind: "operation-removed", Operation: "GET /x", Description: "удалён эндпоинт"},
			}},
		}}, nil)

	var out bytes.Buffer
	require.NoError(t, run(t, publisher, &out))
	assert.Contains(t, out.String(), "frontend v5: ЛОМАЕТ")
	assert.Contains(t, out.String(), "[BREAKING] operation-removed GET /x — удалён эндпоинт")
	assert.Contains(t, out.String(), "ломает часть потребителей")
}

func TestCommandRun_RequiresOneArg(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := NewMockProtocolPublisher(ctrl)
	// Execute не вызывается — аргументы не прошли разбор.

	for _, extra := range [][]string{{}, {"ver", "extra"}} {
		root := rootWith(publisher, &bytes.Buffer{})
		args := append([]string{"paas-cli", "protocols", "publish"}, extra...)
		assert.Error(t, root.Run(context.Background(), args))
	}
}

func TestCommandRun_PropagatesUseCaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := NewMockProtocolPublisher(ctrl)
	publisher.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, entities.ErrInvalidProtocol)

	err := run(t, publisher, &bytes.Buffer{})
	assert.ErrorIs(t, err, entities.ErrInvalidProtocol)
}
