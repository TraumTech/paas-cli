// Package servicefile — транспортная форма файла сервиса и его строгий
// разбор: незнакомый ключ — ошибка с номером строки и подсказкой, а не
// молчаливый пропуск (TRA-554).
package servicefile

// File — весь файл сервиса: манифест и форма читаются из одного файла, поэтому
// допустимые ключи описаны одной структурой.
type File struct {
	Service      *Service     `toml:"service"`
	Protocols    []Protocol   `toml:"protocols"`
	Destination  string       `toml:"destination"`
	Dependencies []Dependency `toml:"dependencies"`

	Processes []Process `toml:"processes"`
	// Databases — потребности в базах (DB-03): [[databases]].
	Databases []Database `toml:"databases"`
	// Buckets — потребности в бакетах (OBJ-06): [[buckets]].
	Buckets []Bucket `toml:"buckets"`
	// Env — секции [env.default] и [env.<окружение>] (DEP-14/15).
	Env map[string]Environment `toml:"env" key:"окружение"`
}

type Service struct {
	Name     string `toml:"name"`
	Contract string `toml:"contract"`
	Format   string `toml:"format"`
}

type Protocol struct {
	Name     string `toml:"name"`
	Contract string `toml:"contract"`
	Format   string `toml:"format"`
}

type Dependency struct {
	Name       string   `toml:"name"`
	Methods    []string `toml:"methods"`
	Waived     []string `toml:"waived"`
	Attributes []string `toml:"attributes"`
}

// Environment — одна секция [env.*].
type Environment struct {
	Variables map[string]string `toml:"variables"`
	// Replicas 0 — секция числа реплик не задаёт.
	Replicas int `toml:"replicas"`
	// Databases — [env.<окружение>.databases.<имя>]: окружение переопределяет
	// только СУБД для объявленной базы.
	Databases map[string]DatabaseOverride `toml:"databases"`
	// Buckets — [env.<окружение>.buckets.<имя>]: хранилище и/или размер.
	Buckets map[string]BucketOverride `toml:"buckets"`
}

type Bucket struct {
	Name   string `toml:"name"`
	Server string `toml:"server"`
	Size   string `toml:"size"`
}

type BucketOverride struct {
	Server string `toml:"server"`
	Size   string `toml:"size"`
}

type Database struct {
	Name   string `toml:"name"`
	Engine string `toml:"engine"`
	// server — имя подключённой СУБД организации, где заводить базу.
	Server string `toml:"server"`
	// variable — переменная с доступом; пусто — умолчание платформы из имени.
	Variable string `toml:"variable"`
}

type DatabaseOverride struct {
	Server string `toml:"server"`
}

type Process struct {
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
	// routes — дополнительные входы на внутренние порты (TRA-509).
	Routes []Route `toml:"routes"`
	// access — правила доступа слушающего порта (TRA-518).
	Access []Access `toml:"access"`
	// cors — сайты, с которых браузеру разрешён основной маршрут (TRA-524).
	CORS CORS `toml:"cors"`
	// aliases — дополнительные адреса основного маршрута (TRA-530).
	Aliases []Alias `toml:"aliases"`
}

type Alias struct {
	Zone   string `toml:"zone"`
	Prefix string `toml:"prefix"`
}

type CORS struct {
	Origins []string `toml:"origins"`
	Headers []string `toml:"headers"`
}

type Access struct {
	Paths     []string `toml:"paths"`
	Methods   []string `toml:"methods"`
	Sources   []string `toml:"sources"`
	Anonymous bool     `toml:"anonymous"`
	// passthrough — вход проверяет сам процесс (TRA-521).
	Passthrough bool `toml:"passthrough"`
}

type Route struct {
	Port      string `toml:"port"`
	Zone      string `toml:"zone"`
	Prefix    string `toml:"prefix"`
	Streaming bool   `toml:"streaming"`
	// access — правила доступа порта этого входа: [[processes.routes.access]].
	Access []Access `toml:"access"`
	// cors — [processes.routes.cors] под своим входом.
	CORS CORS `toml:"cors"`
	// aliases — [[processes.routes.aliases]] под своим входом.
	Aliases []Alias `toml:"aliases"`
}
