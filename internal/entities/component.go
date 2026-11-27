package entities

import (
	"fmt"
	"sort"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// ComponentKindGoModule — вид компонента «Go-модуль»; версия у него — semver.
const ComponentKindGoModule = "go-module"

// Component — то, из чего собран сервис: вид, имя и версия строкой как есть.
type Component struct {
	Kind    string
	Name    string
	Version string
	// Indirect — компонент попал в состав через другой компонент.
	Indirect bool
}

// GoModuleComponents разбирает go.mod в состав сборки: каждый требуемый модуль
// с версией и признаком косвенного. Модуль, подменённый через replace другим
// модулем, идёт в состав под подменяющим именем и версией; подмена локальным
// каталогом версии не несёт, поэтому остаётся исходная пара.
func GoModuleComponents(data []byte) ([]Component, error) {
	file, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidGoMod, err)
	}

	byName := make(map[string]Component, len(file.Require))
	for _, require := range file.Require {
		name, version := require.Mod.Path, require.Mod.Version
		if replaced := replacement(file.Replace, name, version); replaced != nil && replaced.Version != "" {
			name, version = replaced.Path, replaced.Version
		}
		if !semver.IsValid(version) {
			return nil, fmt.Errorf("%w: модуль %s: версия %q", ErrInvalidGoMod, name, version)
		}
		component := Component{Kind: ComponentKindGoModule, Name: name, Version: version, Indirect: require.Indirect}
		// Две подмены могут свести разные модули к одному имени: в сборку
		// попадает старшая версия, прямое требование перевешивает косвенное.
		if seen, dup := byName[name]; dup {
			if semver.Compare(seen.Version, version) > 0 {
				component.Version = seen.Version
			}
			component.Indirect = seen.Indirect && component.Indirect
		}
		byName[name] = component
	}

	components := make([]Component, 0, len(byName))
	for _, c := range byName {
		components = append(components, c)
	}
	sort.Slice(components, func(i, j int) bool { return components[i].Name < components[j].Name })
	return components, nil
}

// replacement — подмена модуля по правилам go: подмена конкретной версии
// точнее общей. nil — модуль не подменён.
func replacement(replaces []*modfile.Replace, path, version string) *module.Version {
	var wildcard *module.Version
	for _, r := range replaces {
		if r.Old.Path != path {
			continue
		}
		if r.Old.Version == version {
			return &r.New
		}
		if r.Old.Version == "" {
			wildcard = &r.New
		}
	}
	return wildcard
}
