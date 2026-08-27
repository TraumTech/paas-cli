package registrydirectoryhttp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/TraumTech/paas-cli/internal/adapters/platformhttp"
	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/pkg/platformapi"
)

// Source читает состояние реестра для гейта исчезнувшего протокола (CLI-23)
// через сгенерированный из контракта клиент (pkg/platformapi): текущие
// протоколы сервиса (PRT-22) и его зарегистрированных потребителей.
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

func (s *Source) ListProtocols(ctx context.Context, serviceID string) ([]entities.RegistryProtocol, error) {
	id, err := uuid.Parse(serviceID)
	if err != nil {
		return nil, fmt.Errorf("неверный id сервиса %q: %w", serviceID, err)
	}
	resp, err := s.client.ListProtocolsWithResponse(ctx, id)
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	switch resp.StatusCode() {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, entities.ErrServiceNotFound
	default:
		return nil, fmt.Errorf("платформа ответила %s", resp.Status())
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("платформа вернула пустой ответ")
	}
	protocols := make([]entities.RegistryProtocol, 0, len(resp.JSON200.Protocols))
	for _, p := range resp.JSON200.Protocols {
		protocols = append(protocols, entities.RegistryProtocol{Name: p.Name, Format: p.Format})
	}
	return protocols, nil
}

func (s *Source) ListConsumers(ctx context.Context, serviceID string) ([]entities.RegisteredConsumer, error) {
	id, err := uuid.Parse(serviceID)
	if err != nil {
		return nil, fmt.Errorf("неверный id сервиса %q: %w", serviceID, err)
	}
	resp, err := s.client.ListProtocolConsumersWithResponse(ctx, id)
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	switch resp.StatusCode() {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, entities.ErrServiceNotFound
	default:
		return nil, fmt.Errorf("платформа ответила %s", resp.Status())
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("платформа вернула пустой ответ")
	}
	consumers := make([]entities.RegisteredConsumer, 0, len(*resp.JSON200))
	for _, c := range *resp.JSON200 {
		consumers = append(consumers, entities.RegisteredConsumer{
			ServiceName:   c.ConsumerServiceName,
			VersionNumber: int(c.ConsumerVersionNumber),
		})
	}
	return consumers, nil
}
