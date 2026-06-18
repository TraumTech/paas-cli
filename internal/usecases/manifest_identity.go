package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// resolveSelfID резолвит имя текущего сервиса (из манифеста) в его id у платформы —
// общий шаг команд, берущих свою идентичность из самодекларации: манифест адресует
// сервис по имени, а платформа — по id.
func resolveSelfID(ctx context.Context, resolver ServiceResolver, name string) (string, error) {
	ids, err := resolver.ResolveIDs(ctx, []string{name})
	if err != nil {
		return "", fmt.Errorf("resolve service: %w", err)
	}
	id, ok := ids[name]
	if !ok {
		return "", fmt.Errorf("сервис %q: %w", name, entities.ErrServiceNotFound)
	}
	return id, nil
}
