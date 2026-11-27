package componentreadergomod_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	componentreadergomod "github.com/TraumTech/paas-cli/internal/adapters/component_reader_gomod"
	"github.com/TraumTech/paas-cli/internal/entities"
)

func TestReadGoMod(t *testing.T) {
	dir := t.TempDir()
	gomod := "module example.org/service\n\ngo 1.26.0\n\nrequire (\n\tgolang.org/x/net v0.60.0 // indirect\n\tgithub.com/google/uuid v1.6.0\n)\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644))

	components, err := componentreadergomod.New().Read(context.Background(), dir)

	require.NoError(t, err)
	require.Equal(t, []entities.Component{
		{Kind: entities.ComponentKindGoModule, Name: "github.com/google/uuid", Version: "v1.6.0"},
		{Kind: entities.ComponentKindGoModule, Name: "golang.org/x/net", Version: "v0.60.0", Indirect: true},
	}, components)
}

// TRA-539/AC-3: сервис не на Go публикуется с пустым составом.
func TestReadWithoutGoMod(t *testing.T) {
	components, err := componentreadergomod.New().Read(context.Background(), t.TempDir())

	require.NoError(t, err)
	require.Empty(t, components)
}

func TestReadInvalidGoMod(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("это не go.mod"), 0o644))

	_, err := componentreadergomod.New().Read(context.Background(), dir)

	require.ErrorIs(t, err, entities.ErrInvalidGoMod)
	require.ErrorContains(t, err, filepath.Join(dir, "go.mod"))
}

// Состав самого CLI читается: настоящий go.mod с косвенными требованиями.
func TestReadOwnGoMod(t *testing.T) {
	components, err := componentreadergomod.New().Read(context.Background(), "../../..")

	require.NoError(t, err)
	require.NotEmpty(t, components)
}
