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
	// Buckets — потребности в бакетах (OBJ-06): [[buckets]].
	Buckets []fileBucket `toml:"buckets"`
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
	// Buckets — [env.<окружение>.buckets.<имя>]: хранилище и/или размер.
	Buckets map[string]fileBucketOverride `toml:"buckets"`
}

type fileBucket struct {
	Name   string `toml:"name"`
	Server string `toml:"server"`
	Size   string `toml:"size"`
}

type fileBucketOverride struct {
	Server string `toml:"server"`
	Size   string `toml:"size"`
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
	// ports — внутренние именованные порты (DEP-24): [processes.ports]
	// с парами «имя = номер»; снаружи недоступны, соседям по кластеру — по имени.
	Ports map[string]int `toml:"ports"`
	// streaming — основной маршрут отдаёт потоковые ответы (TRA-509).
	Streaming bool `toml:"streaming"`
	// routes — дополнительные входы на внутренние порты (TRA-509):
	// [[processes.routes]] с port, zone, prefix, streaming.
	Routes []fileRoute `toml:"routes"`
	// access — правила доступа слушающего порта (TRA-518): [[processes.access]]
	// с paths, methods, sources, anonymous, passthrough.
	Access []fileAccess `toml:"access"`
	// cors — сайты, с которых браузеру разрешён основной маршрут (TRA-524):
	// [processes.cors] с origins и headers.
	CORS fileCORS `toml:"cors"`
}

type fileCORS struct {
	Origins []string `toml:"origins"`
	Headers []string `toml:"headers"`
}

type fileAccess struct {
	Paths     []string `toml:"paths"`
	Methods   []string `toml:"methods"`
	Sources   []string `toml:"sources"`
	Anonymous bool     `toml:"anonymous"`
	// passthrough — вход проверяет сам процесс (TRA-521).
	Passthrough bool `toml:"passthrough"`
}

func accessForms(in []fileAccess) []entities.RouteAccess {
	if len(in) == 0 {
		return nil
	}
	out := make([]entities.RouteAccess, 0, len(in))
	for _, a := range in {
		out = append(out, entities.RouteAccess(a))
	}
	return out
}

type fileRoute struct {
	Port      string `toml:"port"`
	Zone      string `toml:"zone"`
	Prefix    string `toml:"prefix"`
	Streaming bool   `toml:"streaming"`
	// access — правила доступа порта этого входа: [[processes.routes.access]].
	Access []fileAccess `toml:"access"`
	// cors — [processes.routes.cors] под своим входом.
	CORS fileCORS `toml:"cors"`
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
		}
		for _, r := range p.Routes {
			process.Routes = append(process.Routes, entities.RouteForm{
				Port: r.Port, Zone: r.Zone, Prefix: r.Prefix, Streaming: r.Streaming, Access: accessForms(r.Access), CORS: entities.RouteCORS(r.CORS),
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
