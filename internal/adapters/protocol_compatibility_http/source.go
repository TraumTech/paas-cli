package protocolcompatibilityhttp

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

// Source отправляет контракт-кандидат на проверку совместимости в API платформы
// через сгенерированный из контракта клиент (pkg/platformapi). Платформа отвечает
// разбором совместимости с потребителями и ничего не публикует.
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

func (s *Source) CheckCompatibility(ctx context.Context, serviceID string, format entities.ProtocolFormat, document []byte) (*entities.CompatibilityReport, error) {
	id, err := uuid.Parse(serviceID)
	if err != nil {
		return nil, fmt.Errorf("неверный id сервиса %q: %w", serviceID, err)
	}

	// Кандидат уходит в родном для формата виде: OpenAPI — JSON-объект как есть,
	// gRPC — JSON-строка с .proto-исходником. Формат OpenAPI не передаём
	// (умолчание платформы) — запрос не отличается от прежних, без параметра.
	params := &platformapi.CheckProtocolCompatibilityParams{}
	payload := document
	if format == entities.ProtocolFormatGRPC {
		f := platformapi.CheckProtocolCompatibilityParamsFormat(format)
		params.Format = &f
		if payload, err = json.Marshal(string(document)); err != nil {
			return nil, fmt.Errorf("кодирование .proto-кандидата: %w", err)
		}
	}
	resp, err := s.client.CheckProtocolCompatibilityWithBodyWithResponse(ctx, id, params, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	switch resp.StatusCode() {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, entities.ErrServiceNotFound
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return nil, entities.ErrInvalidProtocol
	default:
		return nil, fmt.Errorf("платформа ответила %s", resp.Status())
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("платформа вернула пустой ответ")
	}
	return mapReport(resp.JSON200), nil
}

func mapReport(r *platformapi.CompatibilityReportResponse) *entities.CompatibilityReport {
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
	return &entities.CompatibilityReport{Breaking: r.Breaking, Consumers: consumers}
}
