// Package clusterdirectoryhttp — выходной адаптер: подключённые кластеры
// организации.
package clusterdirectoryhttp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/TraumTech/paas-cli/internal/adapters/platformhttp"
	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/pkg/platformapi"
)

type Directory struct {
	client *platformapi.ClientWithResponses
}

func New(baseURL string, httpClient *http.Client) (*Directory, error) {
	client, err := platformapi.NewClientWithResponses(baseURL, platformapi.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("build platform client: %w", err)
	}
	return &Directory{client: client}, nil
}

func (d *Directory) ListClusters(ctx context.Context) ([]entities.ConnectedCluster, error) {
	resp, err := d.client.ListClustersWithResponse(ctx)
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		return nil, platformhttp.StatusError(resp.StatusCode(), resp.Status(), resp.Body)
	}
	clusters := make([]entities.ConnectedCluster, 0, len(resp.JSON200.Clusters))
	for _, c := range resp.JSON200.Clusters {
		clusters = append(clusters, entities.ConnectedCluster{
			ID:        c.Id.String(),
			Name:      c.Name,
			Endpoint:  c.Endpoint,
			Connected: c.Connected,
		})
	}
	return clusters, nil
}
