// Package clusteraccesshttp — выходной адаптер: какие права платформа просит в
// подключаемом кластере. Список приходит от неё, а не зашит в CLI: иначе он
// устарел бы с первым изменением требований, а команда об этом не узнала бы.
package clusteraccesshttp

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

func (s *Source) RequiredAccess(ctx context.Context) ([]entities.AccessRule, error) {
	resp, err := s.client.GetClusterRequiredAccessWithResponse(ctx)
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		return nil, platformhttp.StatusError(resp.StatusCode(), resp.Status(), resp.Body)
	}

	rules := make([]entities.AccessRule, 0, len(resp.JSON200.Rules))
	for _, rule := range resp.JSON200.Rules {
		rules = append(rules, entities.AccessRule{
			APIGroups: rule.ApiGroups,
			Resources: rule.Resources,
			Verbs:     rule.Verbs,
			Comment:   rule.Comment,
		})
	}
	return rules, nil
}
