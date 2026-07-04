package protocolsourcehttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/pkg/platformapi"
)

// Source тянет опубликованный протокол сервиса из API платформы через
// сгенерированный из контракта клиент (pkg/platformapi). Имя сервиса (для
// раскладки на диске) берётся из GET /services/{id}, сам контракт — из
// GET /services/{id}/protocol.
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

func (s *Source) FetchProtocol(ctx context.Context, serviceID string) (*entities.Protocol, error) {
	id, err := uuid.Parse(serviceID)
	if err != nil {
		return nil, fmt.Errorf("неверный id сервиса %q: %w", serviceID, err)
	}

	svc, err := s.client.GetServiceWithResponse(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("платформа недоступна: %w", err)
	}
	switch svc.StatusCode() {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, entities.ErrServiceNotFound
	default:
		return nil, fmt.Errorf("платформа ответила %s", svc.Status())
	}
	if svc.JSON200 == nil || svc.JSON200.Name == "" {
		return nil, fmt.Errorf("платформа не вернула имя сервиса")
	}

	proto, err := s.client.GetProtocolWithResponse(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("платформа недоступна: %w", err)
	}
	switch proto.StatusCode() {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, entities.ErrServiceNotFound
	default:
		return nil, fmt.Errorf("платформа ответила %s", proto.Status())
	}
	view := proto.JSON200
	if view == nil || !view.Published {
		return nil, entities.ErrProtocolNotPublished
	}

	// Неизвестный CLI формат — честная ошибка, а не контракт, разложенный как
	// попало: старый бинарь не должен молча портить раскладку нового формата.
	formatName := ""
	if view.Format != nil {
		formatName = *view.Format
	}
	format, err := entities.ParseProtocolFormat(formatName)
	if err != nil {
		return nil, err
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
			return nil, fmt.Errorf("сериализация контракта: %w", err)
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
	return protocol, nil
}
