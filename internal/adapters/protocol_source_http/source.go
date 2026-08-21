package protocolsourcehttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/TraumTech/paas-cli/internal/adapters/platformhttp"
	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/pkg/platformapi"
)

// Source тянет опубликованный протокол сервиса из API платформы через
// сгенерированный из контракта клиент (pkg/platformapi). Имя сервиса (для
// раскладки на диске) берётся из GET /services/{id}, сам контракт — из
// GET /services/{id}/protocol. Сужение до методов выполняет платформа (CLI-09):
// methods уходят параметром запроса, CLI получает уже частичный контракт.
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

func (s *Source) FetchProtocol(ctx context.Context, serviceID string, methods, attributes []string) (*entities.Protocol, bool, bool, error) {
	id, err := uuid.Parse(serviceID)
	if err != nil {
		return nil, false, false, fmt.Errorf("неверный id сервиса %q: %w", serviceID, err)
	}

	svc, err := s.client.GetServiceWithResponse(ctx, id)
	if err != nil {
		return nil, false, false, platformhttp.RequestError(err)
	}
	switch svc.StatusCode() {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, false, false, entities.ErrServiceNotFound
	default:
		return nil, false, false, fmt.Errorf("платформа ответила %s", svc.Status())
	}
	if svc.JSON200 == nil || svc.JSON200.Name == "" {
		return nil, false, false, fmt.Errorf("платформа не вернула имя сервиса")
	}

	params := &platformapi.GetProtocolParams{}
	if len(methods) > 0 {
		params.Methods = &methods
	}
	if len(attributes) > 0 {
		params.Attributes = &attributes
	}
	proto, err := s.client.GetProtocolWithResponse(ctx, id, params)
	if err != nil {
		return nil, false, false, platformhttp.RequestError(err)
	}
	switch proto.StatusCode() {
	case http.StatusOK:
	case http.StatusBadRequest:
		// Платформа отклонила сужение (метод или атрибут не найден в контракте) —
		// доносим её сообщение, а не молча неполный срез.
		if p := proto.ApplicationproblemJSONDefault; p != nil && p.Detail != nil && *p.Detail != "" {
			return nil, false, false, fmt.Errorf("платформа отклонила запрос контракта: %s", *p.Detail)
		}
		return nil, false, false, fmt.Errorf("платформа ответила %s", proto.Status())
	case http.StatusNotFound:
		return nil, false, false, entities.ErrServiceNotFound
	default:
		return nil, false, false, fmt.Errorf("платформа ответила %s", proto.Status())
	}
	view := proto.JSON200
	if view == nil || !view.Published {
		return nil, false, false, entities.ErrProtocolNotPublished
	}

	// Неизвестный CLI формат — честная ошибка, а не контракт, разложенный как
	// попало: старый бинарь не должен молча портить раскладку нового формата.
	formatName := ""
	if view.Format != nil {
		formatName = *view.Format
	}
	format, err := entities.ParseProtocolFormat(formatName)
	if err != nil {
		return nil, false, false, err
	}

	// Документ приходит в родном для формата виде: OpenAPI — JSON-объектом в
	// document, gRPC — .proto-исходником в document_text (PRT-17).
	var document []byte
	if format == entities.ProtocolFormatGRPC {
		if view.DocumentText != nil {
			document = []byte(*view.DocumentText)
		}
	} else {
		if document, err = json.Marshal(view.Document); err != nil {
			return nil, false, false, fmt.Errorf("сериализация контракта: %w", err)
		}
	}

	protocol := &entities.Protocol{
		ServiceID:   serviceID,
		ServiceName: svc.JSON200.Name,
		Format:      format,
		Document:    document,
	}
	if view.VersionNumber != nil {
		protocol.VersionNumber = int(*view.VersionNumber)
	}
	narrowingSkipped := view.NarrowingSkipped != nil && *view.NarrowingSkipped
	attributeNarrowingSkipped := view.AttributeNarrowingSkipped != nil && *view.AttributeNarrowingSkipped
	return protocol, narrowingSkipped, attributeNarrowingSkipped, nil
}
