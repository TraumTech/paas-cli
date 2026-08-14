// Package personaltokenhttp — выходной адаптер к личным токенам пользователя на
// платформе (AUTH-20/21). CLI обращается к нему тем же входом, который проверяет:
// перечень доступен только владельцу, поэтому сам факт успешного ответа
// подтверждает, что вход жив.
package personaltokenhttp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/TraumTech/paas-cli/internal/adapters/platformhttp"
	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/pkg/platformapi"
)

type Directory struct {
	client *platformapi.ClientWithResponses
}

func New(baseURL string, httpClient *http.Client) (*Directory, error) {
	client, err := platformapi.NewClientWithResponses(baseURL, platformapi.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("build platform client: %w", err)
	}
	return &Directory{client: client}, nil
}

func (d *Directory) List(ctx context.Context) ([]entities.PersonalToken, error) {
	resp, err := d.client.ListPersonalTokensWithResponse(ctx)
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		return nil, platformhttp.StatusError(resp.StatusCode(), resp.Status(), resp.Body)
	}

	tokens := make([]entities.PersonalToken, 0, len(*resp.JSON200))
	for _, token := range *resp.JSON200 {
		tokens = append(tokens, entities.PersonalToken{
			ID:        token.Id.String(),
			Name:      token.Name,
			ExpiresAt: token.ExpiresAt.UTC(),
		})
	}
	return tokens, nil
}

func (d *Directory) Revoke(ctx context.Context, tokenID string) error {
	id, err := uuid.Parse(tokenID)
	if err != nil {
		return fmt.Errorf("разбор идентификатора токена: %w", err)
	}
	resp, err := d.client.RevokePersonalTokenWithResponse(ctx, id)
	if err != nil {
		return platformhttp.RequestError(err)
	}
	switch resp.StatusCode() {
	case http.StatusNoContent:
		return nil
	// Токен уже отозван (например, из интерфейса) — цель достигнута.
	case http.StatusNotFound:
		return entities.ErrPersonalTokenRejected
	default:
		return platformhttp.StatusError(resp.StatusCode(), resp.Status(), resp.Body)
	}
}
