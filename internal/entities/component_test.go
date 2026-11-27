package entities

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

type goRequire struct {
	path, version string
	indirect      bool
}

func drawModulePath(t *rapid.T, label string) string {
	return rapid.StringMatching(`(github\.com|golang\.org|example\.org)/[a-z][a-z0-9]{0,8}/[a-uw-z][a-z0-9]{0,8}`).Draw(t, label)
}

func drawModuleVersion(t *rapid.T, label string) string {
	return rapid.OneOf(
		rapid.StringMatching(`v[01]\.(0|[1-9][0-9]?)\.(0|[1-9][0-9]?)`),
		rapid.StringMatching(`v[01]\.(0|[1-9][0-9]?)\.(0|[1-9][0-9]?)-rc\.[1-9]`),
		rapid.StringMatching(`v0\.0\.0-202[0-9](0[1-9]|1[0-2])(0[1-9]|1[0-9]|2[0-8])([01][0-9]|2[0-3])[0-5][0-9][0-5][0-9]-[0-9a-f]{12}`),
	).Draw(t, label)
}

func renderGoMod(requires []goRequire, replaces []string) string {
	var b strings.Builder
	b.WriteString("module example.org/service\n\ngo 1.26.0\n\nrequire (\n")
	for _, r := range requires {
		line := "\t" + r.path + " " + r.version
		if r.indirect {
			line += " // indirect"
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(")\n")
	for _, r := range replaces {
		b.WriteString("\nreplace " + r + "\n")
	}
	return b.String()
}

// Каждый требуемый модуль попадает в состав ровно один раз, со своей версией и
// признаком косвенного, в порядке имён — как бы ни был упорядочен go.mod.
func TestGoModuleComponentsMirrorRequires(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		requires := rapid.SliceOfNDistinct(rapid.Custom(func(t *rapid.T) goRequire {
			return goRequire{
				path:     drawModulePath(t, "path"),
				version:  drawModuleVersion(t, "version"),
				indirect: rapid.Bool().Draw(t, "indirect"),
			}
		}), 0, 25, func(r goRequire) string { return r.path }).Draw(t, "requires")

		components, err := GoModuleComponents([]byte(renderGoMod(requires, nil)))
		require.NoError(t, err)

		want := make([]Component, 0, len(requires))
		for _, r := range requires {
			want = append(want, Component{Kind: ComponentKindGoModule, Name: r.path, Version: r.version, Indirect: r.indirect})
		}
		sort.Slice(want, func(i, j int) bool { return want[i].Name < want[j].Name })
		require.Equal(t, want, components)
	})
}

// Подмена модуля другим модулем меняет в составе имя и версию ровно у него;
// подмена локальным каталогом версии не несёт и состав не трогает (TRA-539/AC-2).
func TestGoModuleComponentsApplyReplace(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		requires := rapid.SliceOfNDistinct(rapid.Custom(func(t *rapid.T) goRequire {
			return goRequire{path: drawModulePath(t, "path"), version: drawModuleVersion(t, "version")}
		}), 1, 10, func(r goRequire) string { return r.path }).Draw(t, "requires")
		target := requires[rapid.IntRange(0, len(requires)-1).Draw(t, "target")]
		pinned := rapid.Bool().Draw(t, "pinned")
		local := rapid.Bool().Draw(t, "local")
		forkVersion := drawModuleVersion(t, "fork-version")

		old := target.path
		if pinned {
			old += " " + target.version
		}
		replace := old + " => fork.example/x/y " + forkVersion
		if local {
			replace = old + " => ../local"
		}

		components, err := GoModuleComponents([]byte(renderGoMod(requires, []string{replace})))
		require.NoError(t, err)
		require.Len(t, components, len(requires))

		byName := make(map[string]string, len(components))
		for _, c := range components {
			byName[c.Name] = c.Version
		}
		for _, r := range requires {
			switch {
			case r.path != target.path || local:
				require.Equal(t, r.version, byName[r.path])
			default:
				require.NotContains(t, byName, r.path)
				require.Equal(t, forkVersion, byName["fork.example/x/y"])
			}
		}
	})
}

func TestGoModuleComponentsPinnedReplaceBeatsWildcard(t *testing.T) {
	gomod := renderGoMod(
		[]goRequire{{path: "example.org/a/b", version: "v1.2.3"}},
		[]string{"example.org/a/b v1.2.3 => fork.example/pinned/b v1.2.4", "example.org/a/b => fork.example/any/b v9.0.0"},
	)

	components, err := GoModuleComponents([]byte(gomod))

	require.NoError(t, err)
	require.Equal(t, []Component{{Kind: ComponentKindGoModule, Name: "fork.example/pinned/b", Version: "v1.2.4"}}, components)
}

// Две подмены сводят разные модули к одному имени: в составе оно одно, со
// старшей версией.
func TestGoModuleComponentsMergeReplacedDuplicates(t *testing.T) {
	gomod := renderGoMod(
		[]goRequire{{path: "example.org/a/b", version: "v1.0.0", indirect: true}, {path: "example.org/c/d", version: "v1.0.0"}},
		[]string{"example.org/a/b => fork.example/x/y v1.5.0", "example.org/c/d => fork.example/x/y v1.4.0"},
	)

	components, err := GoModuleComponents([]byte(gomod))

	require.NoError(t, err)
	require.Equal(t, []Component{{Kind: ComponentKindGoModule, Name: "fork.example/x/y", Version: "v1.5.0"}}, components)
}

func TestGoModuleComponentsWithoutRequires(t *testing.T) {
	components, err := GoModuleComponents([]byte("module example.org/service\n\ngo 1.26.0\n"))

	require.NoError(t, err)
	require.Empty(t, components)
}

// TRA-539/AC-4.
func TestGoModuleComponentsRejectUnparsable(t *testing.T) {
	for name, gomod := range map[string]string{
		"мусор": "это не go.mod",
		"требование без версии": "module example.org/service\n\nrequire example.org/a/b\n",
		"версия не semver":      "module example.org/service\n\nrequire example.org/a/b latest\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := GoModuleComponents([]byte(gomod))
			require.ErrorIs(t, err, ErrInvalidGoMod, fmt.Sprintf("%q", gomod))
		})
	}
}
