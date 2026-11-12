// Package registryloginhttp получает у платформы вход в реестр образов
// (POST /registry/login). Ручка моложе снимка контракта в pkg/platformapi,
// поэтому запрос собран вручную.
package registryloginhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/TraumTech/paas-cli/internal/adapters/platformhttp"
	"github.com/TraumTech/paas-cli/internal/entities"
)

type Source struct {
	baseURL string
	client  *http.Client
}

func New(baseURL string, client *http.Client) *Source {
	return &Source{baseURL: baseURL, client: client}
}

func (s *Source) RegistryLogin(ctx context.Context) (*entities.RegistryLogin, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/registry/login", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var problem struct {
			Detail string `json:"detail"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&problem)
		return nil, fmt.Errorf("платформа не выдала вход в реестр (%d): %s", resp.StatusCode, problem.Detail)
	}
	var body struct {
		Server     string    `json:"server"`
		Username   string    `json:"username"`
		Password   string    `json:"password"`
		Repository string    `json:"repository"`
		ExpiresAt  time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("разобрать ответ входа в реестр: %w", err)
	}
	return &entities.RegistryLogin{
		Server: body.Server, Username: body.Username, Password: body.Password,
		Repository: body.Repository, ExpiresAt: body.ExpiresAt,
	}, nil
}
