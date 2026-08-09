// Package formreaderfile читает форму сервиса из paas.toml (DEP-02).
// Отсутствие файла — штатная ветка «формы нет», а не ошибка: публикация без
// формы работает как раньше.
package formreaderfile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/BurntSushi/toml"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type Reader struct{}

func New() *Reader {
	return &Reader{}
}

// fileForm — транспортная форма paas.toml; маппинг в доменную entities.VersionForm
// живёт только здесь, в адаптере.
type fileForm struct {
	Processes []fileProcess `toml:"processes"`
}

type fileProcess struct {
	Name string `toml:"name"`
	// listen — порт, который слушает процесс; отсутствие — воркер.
	Listen  int      `toml:"listen"`
	Command []string `toml:"command"`
	CPU     string   `toml:"cpu"`
	Memory  string   `toml:"memory"`
	// zone/prefix — маршрут слушающего процесса (DEP-10): доменная зона
	// организации по имени и префикс под её базовым хостом.
	Zone   string `toml:"zone"`
	Prefix string `toml:"prefix"`
}

func (r *Reader) Read(_ context.Context, path string) (*entities.VersionForm, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("чтение формы %s: %w", path, err)
	}
	var file fileForm
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("форма %s не разобрана как TOML: %w", path, err)
	}

	// paas.toml — единый манифест (CLI-22): файл есть у каждого сервиса, но
	// форма — только у объявивших процессы. Нет процессов — нет формы.
	if len(file.Processes) == 0 {
		return nil, nil
	}

	form := &entities.VersionForm{}
	for _, p := range file.Processes {
		form.Processes = append(form.Processes, entities.ProcessForm{
			Name:    p.Name,
			Listen:  p.Listen,
			Command: p.Command,
			CPU:     p.CPU,
			Memory:  p.Memory,
			Zone:    p.Zone,
			Prefix:  p.Prefix,
		})
	}
	return form, nil
}
