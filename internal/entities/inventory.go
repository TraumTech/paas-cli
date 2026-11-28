package entities

import (
	"fmt"
	"slices"
	"sort"
)

// PlatformUIURL — интерфейс платформы: туда ведут подсказки «где завести».
const PlatformUIURL = "https://stapel.sh"

// Inventory — что заведено в организации из того, на что ссылается файл
// сервиса: окружения, доменные зоны, СУБД и хранилища (TRA-556).
type Inventory struct {
	Environments []string
	Zones        []string
	Servers      []string
}

// MissingReference — файл сервиса ссылается на то, чего в организации нет.
type MissingReference struct {
	// What — чего нет: «окружение staging».
	What string
	// Where — где в файле на это ссылаются: «секция [env.staging]».
	Where string
	// Remedy — где это завести.
	Remedy string
}

func (m MissingReference) String() string {
	return fmt.Sprintf("%s (%s) — в организации такого нет; %s", m.What, m.Where, m.Remedy)
}

// MissingReferences сверяет ссылки формы с заведённым в организации. Порядок —
// как в файле: окружения, зоны процессов, СУБД баз, хранилища бакетов.
func (d *FormDeclaration) MissingReferences(inventory Inventory) []MissingReference {
	var missing []MissingReference

	environments := make([]string, 0, len(d.Environments))
	for name := range d.Environments {
		if name != DefaultEnvironmentKey {
			environments = append(environments, name)
		}
	}
	sort.Strings(environments)
	for _, name := range environments {
		if !slices.Contains(inventory.Environments, name) {
			missing = append(missing, MissingReference{
				What:   "окружение " + name,
				Where:  fmt.Sprintf("секция [env.%s]", name),
				Remedy: "окружения заводятся на " + PlatformUIURL + "/clusters",
			})
		}
	}

	zone := func(name, where string) {
		if name == "" || slices.Contains(inventory.Zones, name) {
			return
		}
		missing = append(missing, MissingReference{
			What:   "доменная зона " + name,
			Where:  where,
			Remedy: "зоны заводятся на " + PlatformUIURL + "/domains",
		})
	}
	for _, p := range d.Processes {
		zone(p.Zone, fmt.Sprintf("zone процесса %q", p.Name))
		for _, a := range p.Aliases {
			zone(a.Zone, fmt.Sprintf("zone в aliases процесса %q", p.Name))
		}
		for _, r := range p.Routes {
			zone(r.Zone, fmt.Sprintf("zone входа на порт %q процесса %q", r.Port, p.Name))
			for _, a := range r.Aliases {
				zone(a.Zone, fmt.Sprintf("zone в aliases входа на порт %q процесса %q", r.Port, p.Name))
			}
		}
	}

	server := func(name, where string) {
		if name == "" || slices.Contains(inventory.Servers, name) {
			return
		}
		missing = append(missing, MissingReference{
			What:   "СУБД или хранилище " + name,
			Where:  where,
			Remedy: "СУБД и хранилища подключаются на " + PlatformUIURL + "/databases",
		})
	}
	for _, db := range d.Databases {
		server(db.Server, fmt.Sprintf("server базы %q", db.Name))
	}
	for _, b := range d.Buckets {
		server(b.Server, fmt.Sprintf("server бакета %q", b.Name))
	}
	for _, environment := range append([]string{DefaultEnvironmentKey}, environments...) {
		values := d.Environments[environment]
		for _, name := range sortedKeys(values.Databases) {
			server(values.Databases[name].Server, fmt.Sprintf("server в [env.%s.databases.%s]", environment, name))
		}
		for _, name := range sortedKeys(values.Buckets) {
			server(values.Buckets[name].Server, fmt.Sprintf("server в [env.%s.buckets.%s]", environment, name))
		}
	}
	return missing
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
