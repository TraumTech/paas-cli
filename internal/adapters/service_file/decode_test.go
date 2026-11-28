package servicefile

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func unknownKeys(t *testing.T, source string) []UnknownKey {
	t.Helper()
	_, err := Decode("stapel.toml", []byte(source))
	var unknown *UnknownKeysError
	require.ErrorAs(t, err, &unknown)
	return unknown.Keys
}

// TRA-554/AC-1, AC-2: незнакомый ключ — ошибка с ключом, строкой и подсказкой.
func TestDecodeUnknownKeyNamesLineAndSuggestion(t *testing.T) {
	source := "[[processes]]\nname = \"server\"\n\n[env.prod]\nreplica = 2\n"

	_, err := Decode("stapel.toml", []byte(source))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "строка 5: env.prod.replica — возможно, вы имели в виду replicas")
	assert.Equal(t, []UnknownKey{
		{Line: 5, Key: "env.prod.replica", Hint: "возможно, вы имели в виду replicas"},
	}, unknownKeys(t, source))
}

// TRA-554/AC-3: незнакомая секция названа один раз, со своей строкой.
func TestDecodeUnknownSection(t *testing.T) {
	source := "[env.prod.variable]\nA = \"1\"\nB = \"2\"\n"

	_, err := Decode("stapel.toml", []byte(source))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "строка 1: секция [env.prod.variable] — возможно, вы имели в виду variables")
	assert.Len(t, unknownKeys(t, source), 1)
}

// TRA-554/AC-4: перечислены все незнакомые ключи, по порядку строк.
func TestDecodeReportsEveryUnknownKey(t *testing.T) {
	keys := unknownKeys(t, `servise = "x"

[[processes]]
lisen = 8080
replicas = 2
bogus = true

[[processes.routes]]
prot = "admin"

[metrics]
enabled = true
`)

	assert.Equal(t, []UnknownKey{
		{Line: 1, Key: "servise", Hint: "возможно, вы имели в виду service"},
		{Line: 4, Key: "processes.lisen", Hint: "возможно, вы имели в виду listen"},
		{Line: 5, Key: "processes.replicas", Hint: "ключ replicas задаётся в [env.<окружение>]"},
		{Line: 6, Key: "processes.bogus", Hint: "допустимые ключи здесь: name, listen, command, cpu, memory, zone, prefix, ports, streaming, routes, access, cors, aliases"},
		{Line: 9, Key: "processes.routes.prot", Hint: "возможно, вы имели в виду port"},
		{Line: 11, Key: "metrics", Section: true, Hint: "допустимые ключи здесь: service, protocols, destination, dependencies, processes, databases, buckets, env"},
	}, keys)
}

// Свободные имена — переменные, порты, имена окружений и баз — не ключи формата.
func TestDecodeAcceptsFreeNames(t *testing.T) {
	file, err := Decode("stapel.toml", []byte(`
[service]
name = "svc"

[[processes]]
name = "server"
listen = 8080

[processes.ports]
anything = 9000

[env.staging-2.variables]
ANY_NAME = "v"

[env.staging-2.databases.whatever]
server = "main"
`))

	require.NoError(t, err)
	assert.Equal(t, "svc", file.Service.Name)
	assert.Equal(t, map[string]int{"anything": 9000}, file.Processes[0].Ports)
	assert.Equal(t, "v", file.Env["staging-2"].Variables["ANY_NAME"])
}

func TestDecodeWrongValueType(t *testing.T) {
	for source, want := range map[string]string{
		"[[processes]]\nlisten = \"8080\"\n":   "stapel.toml: строка 2: processes.listen — ожидается целое число",
		"[[processes]]\ncommand = \"./app\"\n": "stapel.toml: строка 2: processes.command — ожидается список строк",
		"[env.prod.variables]\nA = 1\n":        "stapel.toml: строка 2: env.prod.variables.A — ожидается строка в кавычках",
		"processes = 1\n":                      "stapel.toml: строка 1: processes — ожидается список секций [[...]]",
	} {
		_, err := Decode("stapel.toml", []byte(source))
		assert.EqualError(t, err, want)
	}
}

func TestDecodeBrokenSyntaxNamesLine(t *testing.T) {
	_, err := Decode("stapel.toml", []byte("[service]\nname = \"a\"\n[[processes\n"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "stapel.toml не разобран как TOML: строка 3")
}

// Файл самого репозитория — действующий пример формата.
func TestDecodeOwnServiceFile(t *testing.T) {
	data, err := os.ReadFile("../../../stapel.toml")
	require.NoError(t, err)

	_, err = Decode("paas.toml", data)

	assert.NoError(t, err)
}
