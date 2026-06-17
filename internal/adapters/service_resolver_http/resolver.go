package serviceresolverhttp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/TraumTech/paas-cli/pkg/platformapi"
)

// Resolver находит id сервисов платформы по именам через список сервисов с
// серверным фильтром (GET /services?name=…): манифест адресует продьюсеров по имени,
// а платформа — по id. Один запрос на весь манифест.
type Resolver struct {
	client *platformapi.ClientWithResponses
}

func New(baseURL string, httpClient *http.Client) (*Resolver, error) {
	client, err := platformapi.NewClientWithResponses(baseURL, platformapi.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("build platform client: %w", err)
	}
	return &Resolver{client: client}, nil
}

func (r *Resolver) ResolveIDs(ctx context.Context, names []string) (map[string]string, error) {
	resp, err := r.client.ListServicesWithResponse(ctx, &platformapi.ListServicesParams{Name: &names})
	if err != nil {
		return nil, fmt.Errorf("платформа недоступна: %w", err)
	}
	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		return nil, fmt.Errorf("платформа ответила %s", resp.Status())
	}
	ids := make(map[string]string, len(*resp.JSON200))
	for _, svc := range *resp.JSON200 {
		ids[svc.Name] = svc.Id.String()
	}
	return ids, nil
}
