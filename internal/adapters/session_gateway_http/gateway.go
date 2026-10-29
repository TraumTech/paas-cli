package sessiongatewayhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// Gateway разговаривает с identity-провайдером платформы (Ory Kratos) по его
// нативному self-service API: проверка сессии (whoami) и её завершение (logout)
// — для входа прежнего вида, сохранённого до IAP-03. Токен сессии предъявляется
// заголовком X-Session-Token — так же CLI ходит с ним и на платформу.
type Gateway struct {
	baseURL string
	client  *http.Client
}

func New(baseURL string, client *http.Client) *Gateway {
	return &Gateway{baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

func (g *Gateway) Inspect(ctx context.Context, token string) (*entities.UserSession, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.baseURL+"/sessions/whoami", nil)
	if err != nil {
		return nil, fmt.Errorf("создание запроса проверки сессии: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Session-Token", token)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("проверка сессии: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var body struct {
			Identity struct {
				Traits struct {
					Email string `json:"email"`
				} `json:"traits"`
			} `json:"identity"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("разбор ответа проверки сессии: %w", err)
		}
		return &entities.UserSession{Token: token, Email: body.Identity.Traits.Email}, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, entities.ErrSessionExpired
	default:
		return nil, fmt.Errorf("проверка сессии: неожиданный ответ провайдера (HTTP %d)", resp.StatusCode)
	}
}

func (g *Gateway) Revoke(ctx context.Context, token string) error {
	payload, err := json.Marshal(map[string]string{"session_token": token})
	if err != nil {
		return fmt.Errorf("подготовка запроса выхода: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, g.baseURL+"/self-service/logout/api", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("создание запроса выхода: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("завершение сессии: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	// Уже недействительный или неизвестный токен — сессии и так нет, цель достигнута.
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return nil
	default:
		return fmt.Errorf("завершение сессии: неожиданный ответ провайдера (HTTP %d)", resp.StatusCode)
	}
}
