package servicefile

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// UnknownKey — ключ файла, которого формат не знает.
type UnknownKey struct {
	Line int
	// Key — полный путь ключа через точку, как его видит TOML.
	Key string
	// Section — ключ объявлен заголовком секции ([a.b] или [[a.b]]).
	Section bool
	Hint    string
}

// UnknownKeysError — файл разобран, но в нём есть незнакомые ключи.
type UnknownKeysError struct {
	Path string
	Keys []UnknownKey
}

func (e *UnknownKeysError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: незнакомые ключи (%d) — файл не принят, чтобы опечатка не потерялась молча", e.Path, len(e.Keys))
	for _, k := range e.Keys {
		if k.Section {
			fmt.Fprintf(&b, "\n  строка %d: секция [%s]", k.Line, k.Key)
		} else {
			fmt.Fprintf(&b, "\n  строка %d: %s", k.Line, k.Key)
		}
		if k.Hint != "" {
			b.WriteString(" — " + k.Hint)
		}
	}
	return b.String()
}

// Decode разбирает файл сервиса строго. path нужен только для текста ошибок.
func Decode(path string, data []byte) (*File, error) {
	var file File
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&file)
	if err == nil {
		return &file, nil
	}
	var strict *toml.StrictMissingError
	if errors.As(err, &strict) {
		unknown := &UnknownKeysError{Path: path}
		lines := bytes.Split(data, []byte("\n"))
		for _, e := range strict.Errors {
			line, _ := e.Position()
			key := []string(e.Key())
			section := line >= 1 && line <= len(lines) && bytes.HasPrefix(bytes.TrimSpace(lines[line-1]), []byte("["))
			unknown.Keys = append(unknown.Keys, UnknownKey{Line: line, Key: strings.Join(key, "."), Section: section, Hint: hint(key)})
		}
		sort.SliceStable(unknown.Keys, func(i, j int) bool { return unknown.Keys[i].Line < unknown.Keys[j].Line })
		return nil, unknown
	}
	var decode *toml.DecodeError
	if errors.As(err, &decode) {
		line, _ := decode.Position()
		if expected := expectation(decode.Key()); expected != "" && strings.Contains(decode.Error(), "cannot decode") {
			return nil, fmt.Errorf("%s: строка %d: %s — ожидается %s", path, line, strings.Join(decode.Key(), "."), expected)
		}
		return nil, fmt.Errorf("%s не разобран как TOML: строка %d: %s", path, line, strings.TrimPrefix(decode.Error(), "toml: "))
	}
	return nil, fmt.Errorf("%s не разобран как TOML: %w", path, err)
}

// expectation называет тип значения известного ключа; пусто — ключ не найден.
func expectation(key []string) string {
	t := reflect.TypeFor[File]()
	for _, segment := range key {
		t = element(t)
		switch t.Kind() {
		case reflect.Map:
			t = t.Elem()
		case reflect.Struct:
			field, ok := fieldByTag(t, segment)
			if !ok {
				return ""
			}
			t = field.Type
		default:
			return ""
		}
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Int:
		return "целое число"
	case reflect.String:
		return "строка в кавычках"
	case reflect.Bool:
		return "true или false"
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Struct {
			return "список секций [[...]]"
		}
		return "список строк"
	case reflect.Map, reflect.Struct:
		return "секция"
	}
	return ""
}

// hint подсказывает, что имелось в виду: близкий по написанию ключ той же
// секции, иначе — секцию, где такой ключ задаётся, иначе — перечень допустимых.
func hint(key []string) string {
	section, name := locate(reflect.TypeFor[File](), key)
	if section == nil {
		return ""
	}
	known := tags(section)
	if len(known) == 0 {
		return ""
	}
	if best := closest(name, known); best != "" {
		return fmt.Sprintf("возможно, вы имели в виду %s", best)
	}
	if homes := homes()[name]; len(homes) > 0 {
		return fmt.Sprintf("ключ %s задаётся в %s", name, strings.Join(homes, ", "))
	}
	return "допустимые ключи здесь: " + strings.Join(known, ", ")
}

// locate идёт по пути ключа вдоль структуры файла и возвращает секцию, где
// путь оборвался, и незнакомое имя в ней.
func locate(t reflect.Type, key []string) (reflect.Type, string) {
	for i := 0; i < len(key); i++ {
		t = element(t)
		switch t.Kind() {
		case reflect.Map:
			t = t.Elem()
		case reflect.Struct:
			field, ok := fieldByTag(t, key[i])
			if !ok {
				return t, key[i]
			}
			t = field.Type
		default:
			return nil, ""
		}
	}
	return nil, ""
}

func element(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	return t
}

func fieldByTag(t reflect.Type, tag string) (reflect.StructField, bool) {
	for i := range t.NumField() {
		if t.Field(i).Tag.Get("toml") == tag {
			return t.Field(i), true
		}
	}
	return reflect.StructField{}, false
}

func tags(t reflect.Type) []string {
	out := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		out = append(out, t.Field(i).Tag.Get("toml"))
	}
	return out
}

// homes — для каждого известного ключа секции, где он задаётся:
// replicas → [env.<окружение>].
var homes = func() func() map[string][]string {
	var index map[string][]string
	return func() map[string][]string {
		if index == nil {
			index = map[string][]string{}
			collect(reflect.TypeFor[File](), "", false, index)
		}
		return index
	}
}()

func collect(t reflect.Type, path string, array bool, index map[string][]string) {
	label := "корне файла"
	if path != "" {
		label = "[" + path + "]"
		if array {
			label = "[" + label + "]"
		}
	}
	for i := range t.NumField() {
		field := t.Field(i)
		tag := field.Tag.Get("toml")
		index[tag] = append(index[tag], label)

		child := strings.TrimPrefix(path+"."+tag, ".")
		ft := field.Type
		isArray := ft.Kind() == reflect.Slice
		ft = element(ft)
		if ft.Kind() == reflect.Map {
			name := field.Tag.Get("key")
			if name == "" {
				name = "имя"
			}
			child += ".<" + name + ">"
			ft = element(ft.Elem())
		}
		if ft.Kind() == reflect.Struct {
			collect(ft, child, isArray, index)
		}
	}
}

// closest возвращает известный ключ, отличающийся от name не более чем на
// треть длины (и не более чем на две правки); пусто — близкого нет.
func closest(name string, known []string) string {
	limit := min(2, max(1, len(name)/3))
	best, bestDistance := "", limit+1
	for _, candidate := range known {
		if d := distance(strings.ToLower(name), candidate); d < bestDistance {
			best, bestDistance = candidate, d
		}
	}
	return best
}

// distance — расстояние Дамерау — Левенштейна: перестановка соседних букв
// считается одной правкой.
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	d := make([][]int, len(ra)+1)
	for i := range d {
		d[i] = make([]int, len(rb)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(ra); i++ {
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(ra)][len(rb)]
}
