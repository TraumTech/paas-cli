package protocolsynccommandcli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

// rootWith собирает корневую команду с глобальным --destination и подкомандой
// `protocols sync`, как это делает app, чтобы тест прогонял реальный разбор аргументов.
func rootWith(syncer ProtocolSyncer, out *bytes.Buffer) *cli.Command {
	return &cli.Command{
		Name:   "paas-cli",
		Writer: out,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: DestinationFlag, Value: "protocols"},
		},
		Commands: []*cli.Command{{
			Name:     "protocols",
			Commands: []*cli.Command{New(syncer).CLICommand()},
		}},
	}
}

func TestCommandRun_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	syncer := NewMockProtocolSyncer(ctrl)
	syncer.EXPECT().
		Execute(gomock.Any(), usecases.SyncProtocolsInput{ManifestPath: "protocols.toml"}).
		Return(&usecases.SyncProtocolsResult{
			Destination: "protocols",
			Protocols: []usecases.FetchProtocolResult{
				{ServiceName: "paas-backend", VersionNumber: 5, Path: "protocols/paas-backend/openapi.json"},
			},
		}, nil)

	var out bytes.Buffer
	root := rootWith(syncer, &out)
	require.NoError(t, root.Run(context.Background(), []string{"paas-cli", "protocols", "sync"}))
	assert.Contains(t, out.String(), "paas-backend")
	assert.Contains(t, out.String(), "получено контрактов — 1")
}

func TestCommandRun_CustomManifest(t *testing.T) {
	ctrl := gomock.NewController(t)
	syncer := NewMockProtocolSyncer(ctrl)
	syncer.EXPECT().
		Execute(gomock.Any(), usecases.SyncProtocolsInput{ManifestPath: "deps/protocols.toml"}).
		Return(&usecases.SyncProtocolsResult{Destination: "protocols"}, nil)

	root := rootWith(syncer, &bytes.Buffer{})
	require.NoError(t, root.Run(context.Background(),
		[]string{"paas-cli", "protocols", "sync", "--manifest", "deps/protocols.toml"}))
}

func TestCommandRun_DestinationOverrideOnlyWhenSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	syncer := NewMockProtocolSyncer(ctrl)
	syncer.EXPECT().
		Execute(gomock.Any(), usecases.SyncProtocolsInput{ManifestPath: "protocols.toml", DestinationOverride: "vendor/api"}).
		Return(&usecases.SyncProtocolsResult{Destination: "vendor/api"}, nil)

	root := rootWith(syncer, &bytes.Buffer{})
	require.NoError(t, root.Run(context.Background(),
		[]string{"paas-cli", "--destination", "vendor/api", "protocols", "sync"}))
}

func TestCommandRun_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	syncer := NewMockProtocolSyncer(ctrl)
	syncer.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

	root := rootWith(syncer, &bytes.Buffer{})
	require.Error(t, root.Run(context.Background(), []string{"paas-cli", "protocols", "sync"}))
}
