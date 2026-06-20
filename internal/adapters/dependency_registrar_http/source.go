package dependencyregistrarhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/pkg/platformapi"
)

// Source регистрирует зависимость версии потребителя от контракта продьюсера через
// сгенерированный из контракта клиент (pkg/platformapi). Платформа идемпотентна:
// повторная регистрация той же версии на того же продьюсера обновляет снимок.
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

func (s *Source) RegisterDependency(ctx context.Context, serviceID, versionID, producerServiceID string, document []byte, methods []string, supersedePrevious bool) (*entities.Dependency, error) {
	id, err := uuid.Parse(serviceID)
	if err != nil {
		return nil, fmt.Errorf("неверный id сервиса %q: %w", serviceID, err)
	}
	versionUUID, err := uuid.Parse(versionID)
	if err != nil {
		return nil, fmt.Errorf("неверный id версии %q: %w", versionID, err)
	}
	producerUUID, err := uuid.Parse(producerServiceID)
	if err != nil {
		return nil, fmt.Errorf("неверный id продьюсера %q: %w", producerServiceID, err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(document, &doc); err != nil {
		return nil, fmt.Errorf("снимок контракта не разобран как JSON: %w", err)
	}

	// nil, когда не замещаем, — поле опускается; платформа трактует отсутствие как false.
	var supersede *bool
	if supersedePrevious {
		supersede = &supersedePrevious
	}
	// Пустой перечень опускаем — платформа трактует отсутствие как «зависит от всего снимка».
	var methodsBody *[]string
	if len(methods) > 0 {
		methodsBody = &methods
	}
	resp, err := s.client.RegisterProtocolDependencyWithResponse(ctx, id, versionUUID, platformapi.RegisterProtocolDependencyJSONRequestBody{
		ProducerServiceId: producerUUID,
		Document:          doc,
		Methods:           methodsBody,
		SupersedePrevious: supersede,
	})
	if err != nil {
		return nil, fmt.Errorf("платформа недоступна: %w", err)
	}
	if resp.StatusCode() != http.StatusCreated {
		// Сервис-потребитель, версия и продьюсер — отдельные сущности, каждая даёт
		// 404, поэтому различаем их сообщением платформы, а не кодом статуса.
		return nil, platformError(resp)
	}
	if resp.JSON201 == nil {
		return nil, fmt.Errorf("платформа вернула пустой ответ")
	}
	return mapDependency(resp.JSON201), nil
}

func platformError(resp *platformapi.RegisterProtocolDependencyResponse) error {
	if p := resp.ApplicationproblemJSONDefault; p != nil {
		if p.Detail != nil && *p.Detail != "" {
			return fmt.Errorf("платформа отклонила регистрацию: %s", *p.Detail)
		}
		if p.Title != nil && *p.Title != "" {
			return fmt.Errorf("платформа отклонила регистрацию: %s", *p.Title)
		}
	}
	return fmt.Errorf("платформа ответила %s", resp.Status())
}

func mapDependency(r *platformapi.ProtocolDependencyResponse) *entities.Dependency {
	return &entities.Dependency{
		ConsumerVersionID: r.ConsumerVersionId.String(),
		ProducerServiceID: r.ProducerServiceId.String(),
		RegisteredAt:      r.RegisteredAt,
	}
}
