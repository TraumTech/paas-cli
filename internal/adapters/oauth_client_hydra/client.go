// Package oauthclienthydra — выходной адаптер к серверу авторизации платформы
// (Ory Hydra) от имени публичного OAuth-клиента CLI (IAP-03): ссылка на вход,
// обмен кода на токены, продление refresh-токеном и отзыв при выходе. Секрета у
// клиента нет — его защищает PKCE и возврат кода только на петлевой адрес.
package oauthclienthydra

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/TraumTech/paas-cli/internal/entities"
)

const (
	authorizePath = "/oauth2/auth"
	tokenPath     = "/oauth2/token"
	revokePath    = "/oauth2/revoke"
	// scopes: openid — чтобы в ответе был id_token (под кем вошли), offline_access —
	// чтобы вход продлевался refresh-токеном без участия человека.
	scopes = "openid offline_access"
)

type Client struct {
	issuer   string
	clientID string
	client   *http.Client
	now      func() time.Time
}

func New(issuer, clientID string, client *http.Client) *Client {
	return &Client{issuer: strings.TrimRight(issuer, "/"), clientID: clientID, client: client, now: time.Now}
}

// AuthorizeURL — адрес, на который CLI ведёт браузер: authorization code + PKCE,
// возврат на redirectURI.
func (c *Client) AuthorizeURL(redirectURI, state, codeChallenge string) string {
	query := url.Values{}
	query.Set("response_type", "code")
	query.Set("client_id", c.clientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", scopes)
	query.Set("state", state)
	query.Set("code_challenge", codeChallenge)
	query.Set("code_challenge_method", "S256")
	return c.issuer + authorizePath + "?" + query.Encode()
}

// Exchange меняет код из браузера на пару токенов.
func (c *Client) Exchange(ctx context.Context, code, codeVerifier, redirectURI string) (*entities.Credential, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("code_verifier", codeVerifier)
	form.Set("redirect_uri", redirectURI)
	credential, err := c.token(ctx, form)
	if err != nil {
		return nil, fmt.Errorf("обмен кода на токены: %w", err)
	}
	return credential, nil
}

// Refresh продлевает вход: сервер выдаёт новую пару, прежний refresh-токен
// гасится. Возвращает entities.ErrSessionExpired, когда продлевать больше
// нечем — refresh истёк, отозван или уже использован.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (*entities.Credential, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	credential, err := c.token(ctx, form)
	if err != nil {
		return nil, fmt.Errorf("продление входа: %w", err)
	}
	return credential, nil
}

// Revoke гасит refresh-токен и выданные по нему токены доступа. Уже негодный
// токен ошибкой не считается: цель — чтобы входа не стало.
func (c *Client) Revoke(ctx context.Context, refreshToken string) error {
	form := url.Values{}
	form.Set("token", refreshToken)
	form.Set("token_type_hint", "refresh_token")
	form.Set("client_id", c.clientID)
	resp, err := c.post(ctx, revokePath, form)
	if err != nil {
		return fmt.Errorf("отзыв входа: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("отзыв входа: неожиданный ответ сервера авторизации (HTTP %d)", resp.StatusCode)
	}
	return nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type tokenError struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

func (c *Client) token(ctx context.Context, form url.Values) (*entities.Credential, error) {
	form.Set("client_id", c.clientID)
	resp, err := c.post(ctx, tokenPath, form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("чтение ответа сервера авторизации: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var failure tokenError
		_ = json.Unmarshal(body, &failure)
		// invalid_grant — код или refresh-токен больше не годятся; для refresh
		// это и есть «вход истёк», для кода — повторить вход.
		if failure.Code == "invalid_grant" {
			return nil, entities.ErrSessionExpired
		}
		if failure.Code != "" {
			return nil, fmt.Errorf("сервер авторизации отказал: %s (%s)", failure.Code, failure.Description)
		}
		return nil, fmt.Errorf("неожиданный ответ сервера авторизации (HTTP %d)", resp.StatusCode)
	}
	var tokens tokenResponse
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("разбор ответа сервера авторизации: %w", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		return nil, errors.New("сервер авторизации не вернул токены")
	}
	return &entities.Credential{
		Kind:         entities.CredentialOIDC,
		Token:        tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		Email:        emailClaim(tokens.IDToken),
		ExpiresAt:    c.now().Add(time.Duration(tokens.ExpiresIn) * time.Second),
	}, nil
}

func (c *Client) post(ctx context.Context, path string, form url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.issuer+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("создание запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("запрос к серверу авторизации: %w", err)
	}
	return resp, nil
}

// emailClaim достаёт e-mail из id_token без проверки подписи: токен только что
// пришёл от самого issuer по TLS, подпись здесь ничего не добавляет, а e-mail
// нужен лишь для подписи «под кем вошли». Пустая строка — claim'а нет.
func emailClaim(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return claims.Email
}
