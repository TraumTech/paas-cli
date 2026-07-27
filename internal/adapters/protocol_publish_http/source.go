package protocolpublishhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/TraumTech/paas-cli/internal/adapters/platformhttp"
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

func (s *Source) PublishProtocol(ctx context.Context, serviceID, versionID string, format entities.ProtocolFormat, document []byte) (*entities.ProtocolPublication, error) {
	id, err := uuid.Parse(serviceID)
	if err != nil {
		return nil, fmt.Errorf("неверный id сервиса %q: %w", serviceID, err)
	}
	versionUUID, err := uuid.Parse(versionID)
	if err != nil {
		return nil, fmt.Errorf("неверный id версии %q: %w", versionID, err)
	}

	// Тело запроса — документ контракта в родном для формата виде: OpenAPI —
	// JSON-объект как есть, gRPC — JSON-строка с .proto-исходником. Формат
	// OpenAPI не передаём (умолчание платформы) — запрос не отличается от
	// прежних публикаций без типа.
	params := &platformapi.PublishProtocolParams{}
	payload := document
	if format == entities.ProtocolFormatGRPC {
		f := platformapi.PublishProtocolParamsFormat(format)
		params.Format = &f
		if payload, err = json.Marshal(string(document)); err != nil {
			return nil, fmt.Errorf("кодирование .proto-контракта: %w", err)
		}
	}

	resp, err := s.client.PublishProtocolWithBodyWithResponse(ctx, id, versionUUID, params, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	// 201 — протокол опубликован впервые, 200 — заменён у версии, где уже был
	// (идемпотентный повтор, штатно при перезапуске выкатки); тело одно и то же.
	var publication *platformapi.ProtocolPublishedResponse
	switch resp.StatusCode() {
	case http.StatusCreated:
		publication = resp.JSON201
	case http.StatusOK:
		publication = resp.JSON200
	default:
		// Сервис и версия — отдельные сегменты пути, оба дают 404, поэтому
		// различаем их сообщением платформы, а не кодом статуса.
		return nil, platformError(resp)
	}
	if publication == nil {
		return nil, fmt.Errorf("платформа вернула пустой ответ")
	}
	return mapPublication(publication), nil
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
			attribute := ""
			if ch.Attribute != nil {
				attribute = *ch.Attribute
			}
			changes = append(changes, entities.CompatibilityChange{
				Breaking:    ch.Breaking,
				Kind:        ch.Kind,
				Operation:   operation,
				Description: ch.Description,
				Attribute:   attribute,
				Waived:      ch.Waived != nil && *ch.Waived,
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
