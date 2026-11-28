// Package organizationinventoryhttp — выходной адаптер: что заведено в
// организации (окружения, доменные зоны, СУБД и хранилища) для сверки файла
// сервиса (TRA-556). Только чтение.
package organizationinventoryhttp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/TraumTech/paas-cli/internal/adapters/platformhttp"
	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/pkg/platformapi"
)

type Source struct {
	client *platformapi.ClientWithResponses
}

func New(baseURL string, httpClient *http.Client) (*Source, error) {
	client, err := platformapi.NewClientWithResponses(baseURL, platformapi.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("build platform client: %w", err)
	}
	return &Source{client: client}, nil
}

func (s *Source) Inventory(ctx context.Context) (*entities.Inventory, error) {
	inventory := &entities.Inventory{}

	environments, err := s.client.ListEnvironmentsWithResponse(ctx)
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	if environments.StatusCode() != http.StatusOK || environments.JSON200 == nil {
		return nil, platformhttp.StatusError(environments.StatusCode(), environments.Status(), environments.Body)
	}
	for _, e := range environments.JSON200.Environments {
		inventory.Environments = append(inventory.Environments, e.Name)
	}

	zones, err := s.client.ListDomainZonesWithResponse(ctx)
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	if zones.StatusCode() != http.StatusOK || zones.JSON200 == nil {
		return nil, platformhttp.StatusError(zones.StatusCode(), zones.Status(), zones.Body)
	}
	for _, z := range zones.JSON200.Zones {
		inventory.Zones = append(inventory.Zones, z.Slug)
	}

	servers, err := s.client.ListDatabaseServersWithResponse(ctx)
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	if servers.StatusCode() != http.StatusOK || servers.JSON200 == nil {
		return nil, platformhttp.StatusError(servers.StatusCode(), servers.Status(), servers.Body)
	}
	for _, server := range servers.JSON200.Servers {
		inventory.Servers = append(inventory.Servers, server.Name)
	}
	return inventory, nil
}
