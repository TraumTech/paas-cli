package app

import (
	"context"
	"errors"
	"testing"

	observability "github.com/TraumTech/paas-observability-sdk"
	"github.com/TraumTech/paas-observability-sdk/apitest"
	"github.com/urfave/cli/v3"
)

func TestInstrumentCommandsWrapsLeafActionWithSpan(t *testing.T) {
	obs := apitest.NewObserver()
	wantErr := errors.New("boom")
	leaf := &cli.Command{Name: "sync", Action: func(context.Context, *cli.Command) error {
		return wantErr
	}}
	group := &cli.Command{Name: "protocols", Commands: []*cli.Command{leaf}}

	instrumentCommands(obs, []*cli.Command{group}, "")

	if err := leaf.Action(context.Background(), leaf); !errors.Is(err, wantErr) {
		t.Fatalf("ошибка команды должна пройти сквозь обёртку, получили %v", err)
	}
	if len(obs.Tracer.Spans) != 1 {
		t.Fatalf("ожидался один спан, получили %d", len(obs.Tracer.Spans))
	}
	span := obs.Tracer.Spans[0]
	if span.Name != "protocols sync" {
		t.Fatalf("имя спана = %q, ожидался путь команды %q", span.Name, "protocols sync")
	}
	if !span.Ended {
		t.Fatal("спан должен завершаться вместе с командой")
	}
	if len(span.Errors) != 1 || !errors.Is(span.Errors[0], wantErr) {
		t.Fatalf("ошибка команды должна попасть в спан, получили %v", span.Errors)
	}
	if span.Status != observability.SpanStatusError {
		t.Fatalf("статус спана = %v, ожидался Error", span.Status)
	}
}

func TestInstrumentCommandsSkipsCommandsWithoutAction(t *testing.T) {
	obs := apitest.NewObserver()
	group := &cli.Command{Name: "protocols"}

	instrumentCommands(obs, []*cli.Command{group}, "")

	if group.Action != nil {
		t.Fatal("команде без Action обёртка не нужна")
	}
}

func TestLogLevelFromEnvDefaultsToWarn(t *testing.T) {
	t.Setenv(envLogLevel, "")
	if got := logLevelFromEnv(); got != observability.LevelWarn {
		t.Fatalf("уровень по умолчанию = %v, ожидался WARN", got)
	}

	t.Setenv(envLogLevel, "DEBUG")
	if got := logLevelFromEnv(); got != observability.LevelDebug {
		t.Fatalf("уровень из окружения = %v, ожидался DEBUG", got)
	}
}
