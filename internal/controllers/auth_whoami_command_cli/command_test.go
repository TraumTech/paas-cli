package authwhoamicommandcli

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

func rootWith(whoami CurrentUser, out *bytes.Buffer) *cli.Command {
	return &cli.Command{
		Name:   "paas-cli",
		Writer: out,
		Commands: []*cli.Command{{
			Name:     "auth",
			Commands: []*cli.Command{New(whoami).CLICommand()},
		}},
	}
}

func TestCommandRun_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	whoami := NewMockCurrentUser(ctrl)
	whoami.EXPECT().Execute(gomock.Any()).Return(&usecases.WhoAmIResult{Email: "user@example.com"}, nil)

	var out bytes.Buffer
	root := rootWith(whoami, &out)
	err := root.Run(context.Background(), []string{"paas-cli", "auth", "whoami"})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Вы вошли как user@example.com")
}

func TestCommandRun_PropagatesUseCaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	whoami := NewMockCurrentUser(ctrl)
	whoami.EXPECT().Execute(gomock.Any()).Return(nil, entities.ErrNoSession)

	root := rootWith(whoami, &bytes.Buffer{})
	err := root.Run(context.Background(), []string{"paas-cli", "auth", "whoami"})

	assert.ErrorIs(t, err, entities.ErrNoSession)
}
