package protocolsourcehttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// Source тянет опубликованный протокол сервиса из HTTP-API платформы. Имя сервиса
// (для раскладки на диске) берётся из GET /services/{id}, сам контракт — из
// GET /services/{id}/protocol.
type Source struct {
	baseURL string
	client  *http.Client
}

func New(baseURL string, client *http.Client) *Source {
	return &Source{baseURL: baseURL, client: client}
}

func (s *Source) FetchProtocol(ctx context.Context, serviceID string) (*entities.Protocol, error) {
	name, err := s.fetchServiceName(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	var view struct {
		VersionNumber int             `json:"version_number"`
		Published     bool            `json:"published"`
		Format        string          `json:"format"`
		Document      json.RawMessage `json:"document"`
	}
	if err := s.getJSON(ctx, fmt.Sprintf("/services/%s/protocol", url.PathEscape(serviceID)), &view); err != nil {
		return nil, err
	}
	if !view.Published {
		return nil, entities.ErrProtocolNotPublished
	}

	return &entities.Protocol{
		ServiceID:     serviceID,
		ServiceName:   name,
		VersionNumber: view.VersionNumber,
		Format:        view.Format,
		Document:      view.Document,
	}, nil
}

func (s *Source) fetchServiceName(ctx context.Context, serviceID string) (string, error) {
	var view struct {
		Name string `json:"name"`
	}
	if err := s.getJSON(ctx, fmt.Sprintf("/services/%s", url.PathEscape(serviceID)), &view); err != nil {
		return "", err
	}
	if view.Name == "" {
		return "", fmt.Errorf("платформа не вернула имя сервиса")
	}
	return view.Name, nil
}

// getJSON выполняет GET и декодирует тело в out; 404 трактуется как «сервис не найден».
func (s *Source) getJSON(ctx context.Context, path string, out any) error {
	endpoint := s.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("платформа недоступна (%s): %w", s.baseURL, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return entities.ErrServiceNotFound
	default:
		return fmt.Errorf("платформа ответила %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("разбор ответа платформы: %w", err)
	}
	return nil
}
