// Package citokengithub — выходной адаптер «токен запуска CI» (TRA-485):
// OIDC-токен GitHub Actions для платформы. Workflow с правом `id-token: write`
// получает адрес и ключ запроса в переменных окружения; токен — короткий JWT,
// подписанный GitHub, его и предъявляет CLI вместо PAAS_API_TOKEN.
package citokengithub

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/TraumTech/paas-cli/internal/entities"
)

const (
	EnvRequestURL   = "ACTIONS_ID_TOKEN_REQUEST_URL"
	EnvRequestToken = "ACTIONS_ID_TOKEN_REQUEST_TOKEN"
)

type Source struct {
	requestURL   string
	requestToken string
	audience     string
	client       *http.Client
}

// New — источник токенов запуска; false — процесс идёт не в GitHub Actions
// или у workflow нет права `id-token: write`.
func New(getenv func(string) string, audience string, client *http.Client) (*Source, bool) {
	requestURL, requestToken := getenv(EnvRequestURL), getenv(EnvRequestToken)
	if requestURL == "" || requestToken == "" {
		return nil, false
	}
	return &Source{requestURL: requestURL, requestToken: requestToken, audience: audience, client: client}, true
}

// Token запрашивает свежий токен запуска для платформы (audience — её API).
func (s *Source) Token(ctx context.Context) (*entities.Credential, error) {
	u, err := url.Parse(s.requestURL)
	if err != nil {
		return nil, fmt.Errorf("адрес запроса токена GitHub Actions: %w", err)
	}
	q := u.Query()
	q.Set("audience", s.audience)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("создание запроса токена GitHub Actions: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.requestToken)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("запрос токена GitHub Actions: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("запрос токена GitHub Actions: неожиданный ответ (HTTP %d)", resp.StatusCode)
	}
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("разбор токена GitHub Actions: %w", err)
	}
	if body.Value == "" {
		return nil, fmt.Errorf("разбор токена GitHub Actions: пустой токен")
	}
	return &entities.Credential{Kind: entities.CredentialCI, Token: body.Value, ExpiresAt: expiresAt(body.Value)}, nil
}

// expiresAt читает срок из самого JWT: подпись проверяет платформа, CLI срок
// нужен лишь для того, чтобы вовремя запросить новый. Не разобрался — нулевой
// срок, токен запрашивается заново перед каждым запросом.
func expiresAt(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return time.Time{}
	}
	return time.Unix(claims.Exp, 0)
}
