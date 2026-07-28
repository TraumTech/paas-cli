// Package clusterregistrarhttp — выходной адаптер: регистрация подключённого
// кластера на платформе. Токен уходит сюда один раз и обратно не возвращается.
package clusterregistrarhttp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/TraumTech/paas-cli/internal/adapters/platformhttp"
	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/pkg/platformapi"
)

type Registrar struct {
	client *platformapi.ClientWithResponses
}

func New(baseURL string, httpClient *http.Client) (*Registrar, error) {
	client, err := platformapi.NewClientWithResponses(baseURL, platformapi.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("build platform client: %w", err)
	}
	return &Registrar{client: client}, nil
}

func (r *Registrar) Register(
	ctx context.Context,
	name string,
	credential entities.ClusterCredential,
) (*entities.ConnectedCluster, error) {
	resp, err := r.client.ConnectClusterWithResponse(ctx, platformapi.ConnectClusterJSONRequestBody{
		Name:          name,
		Endpoint:      credential.Endpoint,
		CaCertificate: credential.CACertificate,
		Token:         credential.Token,
	})
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	if resp.StatusCode() != http.StatusCreated || resp.JSON201 == nil {
		return nil, platformhttp.StatusError(resp.StatusCode(), resp.Status(), resp.Body)
	}

	return &entities.ConnectedCluster{
		ID:        resp.JSON201.Id.String(),
		Name:      resp.JSON201.Name,
		Endpoint:  resp.JSON201.Endpoint,
		Connected: resp.JSON201.Connected,
	}, nil
}
