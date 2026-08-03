package versionpublishcommandcli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

func rootWith(publisher VersionPublisher, out, errOut *bytes.Buffer) *cli.Command {
	return &cli.Command{
		Name:      "paas-cli",
		Writer:    out,
		ErrWriter: errOut,
		Commands: []*cli.Command{{
			Name:     "versions",
			Commands: []*cli.Command{New(publisher).CLICommand()},
		}},
	}
}

func TestCommandRun_PrintsBareIDToStdout(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := NewMockVersionPublisher(ctrl)
	publisher.EXPECT().
		Execute(gomock.Any(), usecases.PublishVersionInput{CommitRevision: "abc123", Environment: "prod", ManifestPath: "", FormPath: "paas.toml"}).
		Return(&entities.Version{ID: "ver-1", Environment: "prod", Number: 7, CommitRevision: "abc123"}, nil)

	var out, errOut bytes.Buffer
	err := rootWith(publisher, &out, &errOut).Run(context.Background(),
		[]string{"paas-cli", "versions", "publish", "abc123"})

	require.NoError(t, err)
	// stdout — только id, без украшений: его подхватывает автоматика.
	assert.Equal(t, "ver-1", strings.TrimSpace(out.String()))
	assert.NotContains(t, out.String(), "Версия")
	// человекочитаемое подтверждение — на stderr.
	assert.Contains(t, errOut.String(), "Версия 7 (prod) зафиксирована для ревизии abc123")
}

func TestCommandRun_RequiresOneArg(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := NewMockVersionPublisher(ctrl)
	// Execute не вызывается — аргументы не прошли разбор.

	for _, extra := range [][]string{{}, {"rev", "extra"}} {
		args := append([]string{"paas-cli", "versions", "publish"}, extra...)
		err := rootWith(publisher, &bytes.Buffer{}, &bytes.Buffer{}).Run(context.Background(), args)
		assert.Error(t, err)
	}
}

func TestCommandRun_PropagatesUseCaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := NewMockVersionPublisher(ctrl)
	publisher.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, entities.ErrServiceNotFound)

	var out bytes.Buffer
	err := rootWith(publisher, &out, &bytes.Buffer{}).Run(context.Background(),
		[]string{"paas-cli", "versions", "publish", "abc123"})

	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
	assert.Empty(t, strings.TrimSpace(out.String()))
}

// Флаг --environment уезжает в use case; дефолт без флага — prod.
func TestCommandRun_ForwardsEnvironment(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := NewMockVersionPublisher(ctrl)
	publisher.EXPECT().
		Execute(gomock.Any(), usecases.PublishVersionInput{CommitRevision: "abc123", Environment: "dev", ManifestPath: "", FormPath: "paas.toml"}).
		Return(&entities.Version{ID: "ver-2", Environment: "dev", Number: 1, CommitRevision: "abc123"}, nil)

	var out, errOut bytes.Buffer
	err := rootWith(publisher, &out, &errOut).Run(context.Background(),
		[]string{"paas-cli", "versions", "publish", "--environment", "dev", "abc123"})

	require.NoError(t, err)
	assert.Equal(t, "ver-2", strings.TrimSpace(out.String()))
	assert.Contains(t, errOut.String(), "Версия 1 (dev)")
}
