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

// fileForm — транспортная форма paas.toml; маппинг в доменное объявление
// entities.FormDeclaration живёт только здесь, в адаптере.
type fileForm struct {
	Processes []fileProcess `toml:"processes"`
	// Databases — потребности в базах (DB-03): [[databases]].
	Databases []fileDatabase `toml:"databases"`
	// Env — секции [env.default] и [env.<окружение>] (DEP-14/15): общие
	// значения и переопределения. Разрешает их use case, зная окружение версии.
	Env map[string]fileEnvironment `toml:"env"`
}

// fileEnvironment — одна секция [env.*].
type fileEnvironment struct {
	Variables map[string]string `toml:"variables"`
	// Replicas 0 — секция числа реплик не задаёт.
	Replicas int `toml:"replicas"`
	// Databases — [env.<окружение>.databases.<имя>]: окружение переопределяет
	// только СУБД для объявленной базы.
	Databases map[string]fileDatabaseOverride `toml:"databases"`
}

type fileDatabase struct {
	Name   string `toml:"name"`
	Engine string `toml:"engine"`
	// server — имя подключённой СУБД организации, где заводить базу.
	Server string `toml:"server"`
	// variable — переменная с доступом; пусто — умолчание платформы из имени.
	Variable string `toml:"variable"`
}

type fileDatabaseOverride struct {
	Server string `toml:"server"`
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

func (r *Reader) Read(_ context.Context, path string) (*entities.FormDeclaration, error) {
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

	declaration := &entities.FormDeclaration{}
	for _, p := range file.Processes {
		declaration.Processes = append(declaration.Processes, entities.ProcessForm{
			Name:    p.Name,
			Listen:  p.Listen,
			Command: p.Command,
			CPU:     p.CPU,
			Memory:  p.Memory,
			Zone:    p.Zone,
			Prefix:  p.Prefix,
		})
	}
	for _, d := range file.Databases {
		declaration.Databases = append(declaration.Databases, entities.DatabaseForm{
			Name:     d.Name,
			Engine:   d.Engine,
			Server:   d.Server,
			Variable: d.Variable,
		})
	}
	for name, values := range file.Env {
		if declaration.Environments == nil {
			declaration.Environments = make(map[string]entities.EnvironmentValues, len(file.Env))
		}
		section := entities.EnvironmentValues{
			Variables: values.Variables,
			Replicas:  values.Replicas,
		}
		for database, override := range values.Databases {
			if section.Databases == nil {
				section.Databases = make(map[string]entities.DatabaseOverride, len(values.Databases))
			}
			section.Databases[database] = entities.DatabaseOverride{Server: override.Server}
		}
		declaration.Environments[name] = section
	}
	return declaration, nil
}
