package dependencyregistrarhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/TraumTech/paas-cli/internal/adapters/platformhttp"
	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
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

func (s *Source) RegisterDependency(ctx context.Context, in usecases.DependencyRegistration) (*entities.Dependency, error) {
	id, err := uuid.Parse(in.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("неверный id сервиса %q: %w", in.ServiceID, err)
	}
	versionUUID, err := uuid.Parse(in.VersionID)
	if err != nil {
		return nil, fmt.Errorf("неверный id версии %q: %w", in.VersionID, err)
	}
	producerUUID, err := uuid.Parse(in.ProducerServiceID)
	if err != nil {
		return nil, fmt.Errorf("неверный id продьюсера %q: %w", in.ProducerServiceID, err)
	}

	// Снимок уходит в родном виде формата (PRT-19): OpenAPI — JSON-объект,
	// gRPC — строка с .proto-исходником. Формат прежнего типа не передаём —
	// запрос не отличается от прежних регистраций без типа.
	var doc any
	var formatBody *platformapi.RegisterProtocolDependencyInputBodyFormat
	if in.Format == entities.ProtocolFormatGRPC {
		doc = string(in.Document)
		f := platformapi.RegisterProtocolDependencyInputBodyFormat(in.Format)
		formatBody = &f
	} else {
		var obj map[string]any
		if err := json.Unmarshal(in.Document, &obj); err != nil {
			return nil, fmt.Errorf("снимок контракта не разобран как JSON: %w", err)
		}
		doc = obj
	}

	// nil, когда не замещаем, — поле опускается; платформа трактует отсутствие как false.
	var supersede *bool
	if in.SupersedePrevious {
		supersede = &in.SupersedePrevious
	}
	// Пустой перечень опускаем — платформа трактует отсутствие как «зависит от всего снимка».
	var methodsBody *[]string
	if len(in.Methods) > 0 {
		methodsBody = &in.Methods
	}
	// Пустой перечень отказов опускаем — платформа трактует отсутствие как «отказов нет».
	var waivedBody *[]string
	if len(in.Waived) > 0 {
		waivedBody = &in.Waived
	}
	resp, err := s.client.RegisterProtocolDependencyWithResponse(ctx, id, versionUUID, platformapi.RegisterProtocolDependencyJSONRequestBody{
		ProducerServiceId: producerUUID,
		Format:            formatBody,
		Document:          doc,
		Methods:           methodsBody,
		WaivedAttributes:  waivedBody,
		SupersedePrevious: supersede,
	})
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	// 201 — первая регистрация, 200 — снимок заменён (идемпотентный повтор при
	// перезапуске выкатки); тело в обоих случаях одно.
	var out *platformapi.ProtocolDependencyResponse
	switch resp.StatusCode() {
	case http.StatusCreated:
		out = resp.JSON201
	case http.StatusOK:
		out = resp.JSON200
	default:
		// Сервис-потребитель, версия и продьюсер — отдельные сущности, каждая даёт
		// 404, поэтому различаем их сообщением платформы, а не кодом статуса.
		return nil, platformError(resp)
	}
	if out == nil {
		return nil, fmt.Errorf("платформа вернула пустой ответ")
	}
	return mapDependency(out), nil
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
