// Package servicefile находит файл сервиса в репозитории: stapel.toml, а при
// его отсутствии — прежний paas.toml.
package servicefile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	Name       = "stapel.toml"
	FormerName = "paas.toml"
)

// Warnings — куда уходит предупреждение о лишнем прежнем файле.
var Warnings io.Writer = os.Stderr

var warnOnce sync.Once

// Resolve выбирает файл сервиса в каталоге dir; файла нет — путь, где его
// ждут. При обоих файлах читается
// stapel.toml: правки в прежнем молча не действовали бы, поэтому о нём
// предупреждаем.
func Resolve(dir string) (string, bool) {
	current, former := filepath.Join(dir, Name), filepath.Join(dir, FormerName)
	if !exists(current) {
		if exists(former) {
			return former, true
		}
		return current, false
	}
	if exists(former) {
		warnOnce.Do(func() {
			fmt.Fprintf(Warnings, "предупреждение: найдены %s и %s — читается %s, %s не используется, удалите его\n",
				Name, FormerName, Name, FormerName)
		})
	}
	return current, true
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
