// Package platformhttp — общее для HTTP-адаптеров платформы: перевод
// транспортных ошибок запроса на язык пользователя.
package platformhttp

import (
	"errors"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// RequestError оборачивает ошибку неудавшегося запроса к платформе. Доменную
// ошибку (например, отказ аутентификации, сформулированный транспортом
// composition root) возвращаем как есть — без обёртки http-клиента с URL,
// которая маскировала бы готовую подсказку пользователю.
func RequestError(err error) error {
	var domainErr *entities.DomainError
	if errors.As(err, &domainErr) {
		return domainErr
	}
	return fmt.Errorf("платформа недоступна: %w", err)
}
