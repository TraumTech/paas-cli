package versionpublisherhttp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/pkg/platformapi"
)

// Source фиксирует версию сервиса в API платформы через сгенерированный из
// контракта клиент (pkg/platformapi). Платформа идемпотентна: одна ревизия — одна
// версия, повторный вызов возвращает ту же версию.
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

func (s *Source) PublishVersion(ctx context.Context, serviceID, commitRevision string) (*entities.Version, error) {
	id, err := uuid.Parse(serviceID)
	if err != nil {
		return nil, fmt.Errorf("неверный id сервиса %q: %w", serviceID, err)
	}

	resp, err := s.client.PublishVersionWithResponse(ctx, id, platformapi.PublishVersionJSONRequestBody{
		CommitRevision: commitRevision,
	})
	if err != nil {
		return nil, fmt.Errorf("платформа недоступна: %w", err)
	}
	switch resp.StatusCode() {
	case http.StatusCreated:
	case http.StatusNotFound:
		return nil, entities.ErrServiceNotFound
	default:
		return nil, fmt.Errorf("платформа ответила %s", resp.Status())
	}
	if resp.JSON201 == nil {
		return nil, fmt.Errorf("платформа вернула пустой ответ")
	}
	return mapVersion(resp.JSON201), nil
}

func mapVersion(v *platformapi.VersionResponse) *entities.Version {
	return &entities.Version{
		ID:             v.Id.String(),
		Number:         int(v.Number),
		CommitRevision: v.CommitRevision,
		CreatedAt:      v.CreatedAt,
	}
}
