// Package userinfohydra — выходной адаптер «под кем выполнен вход» для входа
// OIDC (IAP-03): ручка userinfo сервера авторизации платформы. Ходит клиентом с
// сохранённым входом: сам ответ подтверждает, что токен доступа принят, а claim'ы
// в нём — те, что сервер положил в id_token при согласии.
package userinfohydra

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/TraumTech/paas-cli/internal/entities"
)

const userInfoPath = "/userinfo"

type Inspector struct {
	issuer string
	client *http.Client
}

func New(issuer string, client *http.Client) *Inspector {
	return &Inspector{issuer: strings.TrimRight(issuer, "/"), client: client}
}

func (i *Inspector) Inspect(ctx context.Context) (*entities.UserSession, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.issuer+userInfoPath, nil)
	if err != nil {
		return nil, fmt.Errorf("создание запроса проверки входа: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := i.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("проверка входа: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		var body struct {
			Subject string `json:"sub"`
			Email   string `json:"email"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("разбор ответа проверки входа: %w", err)
		}
		return &entities.UserSession{Email: body.Email, UserID: body.Subject}, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, entities.ErrSessionExpired
	default:
		return nil, fmt.Errorf("проверка входа: неожиданный ответ сервера авторизации (HTTP %d)", resp.StatusCode)
	}
}
