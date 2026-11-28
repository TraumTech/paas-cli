package usecases

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// InventorySource отдаёт заведённое в организации, с чем сверяется файл
// сервиса (TRA-556). Только читает.
type InventorySource interface {
	Inventory(ctx context.Context) (*entities.Inventory, error)
}
