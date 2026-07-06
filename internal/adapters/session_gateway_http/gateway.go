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
// нативному self-service API: вход по паролю (login flow), проверка сессии
// (whoami) и её завершение (logout). Токен сессии предъявляется заголовком
// X-Session-Token — так же CLI ходит и на платформу, где сессию валидирует прокси.
type Gateway struct {
	baseURL string
	client  *http.Client
}

func New(baseURL string, client *http.Client) *Gateway {
	return &Gateway{baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

func (g *Gateway) Authenticate(ctx context.Context, email, password string) (*entities.UserSession, error) {
	flowID, err := g.createLoginFlow(ctx)
	if err != nil {
		return nil, err
	}
	return g.submitLoginFlow(ctx, flowID, email, password)
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

// createLoginFlow заводит нативный (API) login flow и возвращает его id.
func (g *Gateway) createLoginFlow(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.baseURL+"/self-service/login/api", nil)
	if err != nil {
		return "", fmt.Errorf("создание запроса login flow: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("создание login flow: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("создание login flow: неожиданный ответ провайдера (HTTP %d)", resp.StatusCode)
	}
	var flow struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&flow); err != nil {
		return "", fmt.Errorf("разбор login flow: %w", err)
	}
	if flow.ID == "" {
		return "", fmt.Errorf("создание login flow: провайдер не вернул id")
	}
	return flow.ID, nil
}

func (g *Gateway) submitLoginFlow(ctx context.Context, flowID, email, password string) (*entities.UserSession, error) {
	payload, err := json.Marshal(map[string]string{
		"method":     "password",
		"identifier": email,
		"password":   password,
	})
	if err != nil {
		return nil, fmt.Errorf("подготовка учётных данных: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/self-service/login?flow="+flowID, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("создание запроса входа: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("вход: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var body struct {
			SessionToken string `json:"session_token"`
			Session      struct {
				Identity struct {
					Traits struct {
						Email string `json:"email"`
					} `json:"traits"`
				} `json:"identity"`
			} `json:"session"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("разбор ответа входа: %w", err)
		}
		if body.SessionToken == "" {
			return nil, fmt.Errorf("вход: провайдер не вернул токен сессии")
		}
		return &entities.UserSession{Token: body.SessionToken, Email: body.Session.Identity.Traits.Email}, nil
	// 400 у нативного login flow — учётные данные не признаны; детали (что именно
	// неверно) пользователю сознательно не раскрываем.
	case http.StatusBadRequest, http.StatusUnauthorized:
		return nil, entities.ErrInvalidCredentials
	default:
		return nil, fmt.Errorf("вход: неожиданный ответ провайдера (HTTP %d)", resp.StatusCode)
	}
}
