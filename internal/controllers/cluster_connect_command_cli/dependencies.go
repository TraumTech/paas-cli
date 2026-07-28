package clusterconnectcommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=clusterconnectcommandcli github.com/TraumTech/paas-cli/internal/controllers/cluster_connect_command_cli ClusterConnector

type ClusterConnector interface {
	Execute(ctx context.Context, input usecases.ConnectClusterInput, confirm usecases.ConfirmFunc) (*entities.ConnectedCluster, error)
}
