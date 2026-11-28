// Package formreaderfile читает форму сервиса из файла сервиса (DEP-02).
// Отсутствие файла — штатная ветка «формы нет», а не ошибка: публикация без
// формы работает как раньше.
package formreaderfile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	servicefile "github.com/TraumTech/paas-cli/internal/adapters/service_file"
	"github.com/TraumTech/paas-cli/internal/entities"
)

type Reader struct{}

func New() *Reader {
	return &Reader{}
}

func aliasForms(in []servicefile.Alias) []entities.RouteAlias {
	if len(in) == 0 {
		return nil
	}
	out := make([]entities.RouteAlias, 0, len(in))
	for _, a := range in {
		out = append(out, entities.RouteAlias(a))
	}
	return out
}

func accessForms(in []servicefile.Access) []entities.RouteAccess {
	if len(in) == 0 {
		return nil
	}
	out := make([]entities.RouteAccess, 0, len(in))
	for _, a := range in {
		out = append(out, entities.RouteAccess(a))
	}
	return out
}

// Read читает форму; пустой путь — файл сервиса в текущем каталоге.
func (r *Reader) Read(_ context.Context, path string) (*entities.FormDeclaration, error) {
	if path == "" {
		path, _ = servicefile.Resolve(".")
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("чтение формы %s: %w", path, err)
	}
	file, err := servicefile.Decode(path, data)
	if err != nil {
		return nil, err
	}

	// Файл сервиса — единый манифест (CLI-22): файл есть у каждого сервиса, но
	// форма — только у объявивших процессы. Нет процессов — нет формы.
	if len(file.Processes) == 0 {
		return nil, nil
	}

	declaration := &entities.FormDeclaration{}
	for _, p := range file.Processes {
		process := entities.ProcessForm{
			Name:      p.Name,
			Listen:    p.Listen,
			Command:   p.Command,
			CPU:       p.CPU,
			Memory:    p.Memory,
			Zone:      p.Zone,
			Prefix:    p.Prefix,
			Ports:     p.Ports,
			Streaming: p.Streaming,
			Access:    accessForms(p.Access),
			CORS:      entities.RouteCORS(p.CORS),
			Aliases:   aliasForms(p.Aliases),
		}
		for _, r := range p.Routes {
			process.Routes = append(process.Routes, entities.RouteForm{
				Port: r.Port, Zone: r.Zone, Prefix: r.Prefix, Streaming: r.Streaming, Access: accessForms(r.Access), CORS: entities.RouteCORS(r.CORS), Aliases: aliasForms(r.Aliases),
			})
		}
		declaration.Processes = append(declaration.Processes, process)
	}
	for _, d := range file.Databases {
		declaration.Databases = append(declaration.Databases, entities.DatabaseForm{
			Name:     d.Name,
			Engine:   d.Engine,
			Server:   d.Server,
			Variable: d.Variable,
		})
	}
	for _, b := range file.Buckets {
		declaration.Buckets = append(declaration.Buckets, entities.BucketForm{Name: b.Name, Server: b.Server, Size: b.Size})
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
		for bucket, override := range values.Buckets {
			if section.Buckets == nil {
				section.Buckets = make(map[string]entities.BucketOverride, len(values.Buckets))
			}
			section.Buckets[bucket] = entities.BucketOverride{Server: override.Server, Size: override.Size}
		}
		declaration.Environments[name] = section
	}
	return declaration, nil
}
