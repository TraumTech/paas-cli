package candidatereaderfile

import (
	"context"
	"fmt"
	"os"
)

// Reader читает документ контракта-кандидата из файла на диске потребителя.
type Reader struct{}

func New() *Reader {
	return &Reader{}
}

func (r *Reader) Read(_ context.Context, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение файла кандидата %s: %w", path, err)
	}
	return data, nil
}
