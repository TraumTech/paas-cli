package dependencyregistercommandcli

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

func rootWith(registrar DependencyRegistrar, out, errOut *bytes.Buffer) *cli.Command {
	return &cli.Command{
		Name:      "paas-cli",
		Writer:    out,
		ErrWriter: errOut,
		Commands: []*cli.Command{{
			Name:     "dependencies",
			Commands: []*cli.Command{New(registrar).CLICommand()},
		}},
	}
}

func TestCommandRun_RegistersAndConfirms(t *testing.T) {
	ctrl := gomock.NewController(t)
	registrar := NewMockDependencyRegistrar(ctrl)
	registrar.EXPECT().
		Execute(gomock.Any(), usecases.RegisterDependencyInput{VersionID: "ver-1", ManifestPath: "protocols.toml"}).
		Return(&usecases.RegisterDependenciesResult{Registered: []usecases.RegisteredDependency{
			{ProducerName: "paas-backend", ProducerServiceID: "prod"},
		}}, nil)

	var out, errOut bytes.Buffer
	err := rootWith(registrar, &out, &errOut).Run(context.Background(),
		[]string{"paas-cli", "dependencies", "register", "ver-1"})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Зависимость от paas-backend (prod) зарегистрирована")
	assert.Contains(t, out.String(), "зарегистрировано зависимостей — 1")
}

func TestCommandRun_RequiresOneArg(t *testing.T) {
	ctrl := gomock.NewController(t)
	registrar := NewMockDependencyRegistrar(ctrl)
	// Execute не вызывается — аргументы не прошли разбор.

	for _, extra := range [][]string{{}, {"ver", "extra"}} {
		args := append([]string{"paas-cli", "dependencies", "register"}, extra...)
		err := rootWith(registrar, &bytes.Buffer{}, &bytes.Buffer{}).Run(context.Background(), args)
		assert.Error(t, err)
	}
}

func TestCommandRun_PropagatesUseCaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	registrar := NewMockDependencyRegistrar(ctrl)
	registrar.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, entities.ErrServiceNotFound)

	var out bytes.Buffer
	err := rootWith(registrar, &out, &bytes.Buffer{}).Run(context.Background(),
		[]string{"paas-cli", "dependencies", "register", "ver-1"})

	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
	assert.Empty(t, strings.TrimSpace(out.String()))
}
