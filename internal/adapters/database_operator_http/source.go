// Package databaseoperatorhttp — выходной адаптер: оператор СУБД от платформы
// (манифест и право, нужное ей после установки).
package databaseoperatorhttp

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

func (s *Source) Operator(ctx context.Context, engine string) (*entities.DatabaseOperator, error) {
	resp, err := s.client.GetDatabaseOperatorWithResponse(ctx, platformapi.GetDatabaseOperatorParamsEngine(engine))
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, entities.ErrUnknownEngine
	}
	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		return nil, platformhttp.StatusError(resp.StatusCode(), resp.Status(), resp.Body)
	}

	op := resp.JSON200
	rules := make([]entities.AccessRule, 0, len(op.Rules))
	for _, rule := range op.Rules {
		rules = append(rules, entities.AccessRule{
			APIGroups: rule.ApiGroups,
			Resources: rule.Resources,
			Verbs:     rule.Verbs,
			Comment:   rule.Comment,
		})
	}
	return &entities.DatabaseOperator{
		Engine:     string(op.Engine),
		Name:       op.Name,
		Version:    op.Version,
		Manifest:   op.Manifest,
		Namespace:  op.Namespace,
		Deployment: op.Deployment,
		Rules:      rules,
	}, nil
}
