package protocolpublishhttp

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/pkg/platformapi"
)

// Source публикует контракт под версией сервиса через сгенерированный из контракта
// клиент (pkg/platformapi). Платформа привязывает протокол к версии и возвращает
// разбор совместимости с потребителями.
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

func (s *Source) PublishProtocol(ctx context.Context, serviceID, versionID string, document []byte) (*entities.ProtocolPublication, error) {
	id, err := uuid.Parse(serviceID)
	if err != nil {
		return nil, fmt.Errorf("неверный id сервиса %q: %w", serviceID, err)
	}
	versionUUID, err := uuid.Parse(versionID)
	if err != nil {
		return nil, fmt.Errorf("неверный id версии %q: %w", versionID, err)
	}

	resp, err := s.client.PublishProtocolWithBodyWithResponse(ctx, id, versionUUID, "application/json", bytes.NewReader(document))
	if err != nil {
		return nil, fmt.Errorf("платформа недоступна: %w", err)
	}
	if resp.StatusCode() != http.StatusCreated {
		// Сервис и версия — отдельные сегменты пути, оба дают 404, поэтому
		// различаем их сообщением платформы, а не кодом статуса.
		return nil, platformError(resp)
	}
	if resp.JSON201 == nil {
		return nil, fmt.Errorf("платформа вернула пустой ответ")
	}
	return mapPublication(resp.JSON201), nil
}

func platformError(resp *platformapi.PublishProtocolResponse) error {
	if p := resp.ApplicationproblemJSONDefault; p != nil {
		if p.Detail != nil && *p.Detail != "" {
			return fmt.Errorf("платформа отклонила публикацию: %s", *p.Detail)
		}
		if p.Title != nil && *p.Title != "" {
			return fmt.Errorf("платформа отклонила публикацию: %s", *p.Title)
		}
	}
	return fmt.Errorf("платформа ответила %s", resp.Status())
}

func mapPublication(r *platformapi.ProtocolPublishedResponse) *entities.ProtocolPublication {
	versionNumber := 0
	if r.Protocol != nil {
		versionNumber = int(r.Protocol.VersionNumber)
	}
	consumers := make([]entities.ConsumerCompatibility, 0, len(r.Consumers))
	for _, c := range r.Consumers {
		changes := make([]entities.CompatibilityChange, 0, len(c.Changes))
		for _, ch := range c.Changes {
			operation := ""
			if ch.Operation != nil {
				operation = *ch.Operation
			}
			changes = append(changes, entities.CompatibilityChange{
				Breaking:    ch.Breaking,
				Kind:        ch.Kind,
				Operation:   operation,
				Description: ch.Description,
			})
		}
		consumers = append(consumers, entities.ConsumerCompatibility{
			ServiceName:   c.ConsumerServiceName,
			VersionNumber: int(c.ConsumerVersionNumber),
			Comparable:    c.Comparable,
			Breaking:      c.Breaking,
			Changes:       changes,
		})
	}
	return &entities.ProtocolPublication{
		VersionNumber: versionNumber,
		Breaking:      r.Breaking,
		Consumers:     consumers,
	}
}
