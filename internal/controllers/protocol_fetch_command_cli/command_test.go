package protocolfetchcommandcli

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

// rootWith собирает корневую команду с глобальным --destination и подкомандой
// `protocols fetch`, как это делает app — чтобы тест прогонял реальный разбор
// аргументов urfave/cli (включая чтение глобального флага через lineage).
func rootWith(fetcher ProtocolFetcher, out *bytes.Buffer) *cli.Command {
	return &cli.Command{
		Name:   "paas-cli",
		Writer: out,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: DestinationFlag, Value: "protocols"},
		},
		Commands: []*cli.Command{{
			Name:     "protocols",
			Commands: []*cli.Command{New(fetcher).CLICommand()},
		}},
	}
}

func TestCommandRun_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	fetch := NewMockProtocolFetcher(ctrl)
	fetch.EXPECT().
		Execute(gomock.Any(), usecases.FetchProtocolInput{ServiceID: "svc-1", Destination: "protocols"}).
		Return(&usecases.FetchProtocolResult{ServiceName: "payments", VersionNumber: 7, Path: "protocols/payments/openapi.json"}, nil)

	var out bytes.Buffer
	root := rootWith(fetch, &out)
	err := root.Run(context.Background(), []string{"paas-cli", "protocols", "fetch", "svc-1"})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "payments")
	assert.Contains(t, out.String(), "protocols/payments/openapi.json")
}

func TestCommandRun_CustomDestination(t *testing.T) {
	ctrl := gomock.NewController(t)
	fetch := NewMockProtocolFetcher(ctrl)
	fetch.EXPECT().
		Execute(gomock.Any(), usecases.FetchProtocolInput{ServiceID: "svc-1", Destination: "vendor/api"}).
		Return(&usecases.FetchProtocolResult{ServiceName: "payments"}, nil)

	root := rootWith(fetch, &bytes.Buffer{})
	require.NoError(t, root.Run(context.Background(),
		[]string{"paas-cli", "--destination", "vendor/api", "protocols", "fetch", "svc-1"}))
}

func TestCommandRun_PartialPassesMethods(t *testing.T) {
	ctrl := gomock.NewController(t)
	fetch := NewMockProtocolFetcher(ctrl)
	fetch.EXPECT().
		Execute(gomock.Any(), usecases.FetchProtocolInput{ServiceID: "svc-1", Destination: "protocols", Methods: []string{"op-a", "op-b"}}).
		Return(&usecases.FetchProtocolResult{ServiceName: "payments", Path: "protocols/payments/openapi.json"}, nil)

	var out bytes.Buffer
	root := rootWith(fetch, &out)
	err := root.Run(context.Background(),
		[]string{"paas-cli", "protocols", "fetch", "svc-1", "--method", "op-a", "--method", "op-b"})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "частичный")
}

func TestCommandRun_RequiresExactlyOneServiceID(t *testing.T) {
	ctrl := gomock.NewController(t)
	// Execute не вызывается — аргументы не прошли разбор.
	fetch := NewMockProtocolFetcher(ctrl)

	for _, extra := range [][]string{{}, {"a", "b"}} {
		root := rootWith(fetch, &bytes.Buffer{})
		args := append([]string{"paas-cli", "protocols", "fetch"}, extra...)
		assert.Error(t, root.Run(context.Background(), args))
	}
}

func TestCommandRun_PropagatesUseCaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	fetch := NewMockProtocolFetcher(ctrl)
	fetch.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, entities.ErrProtocolNotPublished)

	root := rootWith(fetch, &bytes.Buffer{})
	err := root.Run(context.Background(), []string{"paas-cli", "protocols", "fetch", "svc-1"})
	assert.ErrorIs(t, err, entities.ErrProtocolNotPublished)
}
