package app

import (
	"context"
	"os"
	"strings"

	observability "github.com/TraumTech/paas-observability-sdk"
	"github.com/TraumTech/paas-observability-sdk/sdk/observabilityotel"
	"github.com/TraumTech/paas-observability-sdk/sdk/observabilityzerolog"
	"github.com/urfave/cli/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

const (
	obsServiceName = "paas-cli"
	obsEnvironment = "production"
	// envOTLPURL — приёмный OTLP/HTTP-шлюз платформы (тот же, куда шлёт трассы
	// браузер, OBS-03); значение `off` выключает трассировку.
	envOTLPURL     = "PAAS_OTLP_URL"
	defaultOTLPURL = "https://otlp.paas.traumtech.ru"
	otlpURLOff     = "off"
	// envLogLevel/envLogEncoding — те же переменные, что у observabilitydefault.
	envLogLevel    = "OBSERVABILITY_LOG_LEVEL"
	envLogEncoding = "OBSERVABILITY_LOG_ENCODING"
)

// newObserver собирает Observer для CLI из компонентов SDK вручную:
// observabilitydefault не подходит короткоживущему процессу на машине
// пользователя — он поднимает Prometheus HTTP-сервер и экспортирует трассы
// только по OTLP gRPC, а внешний шлюз платформы принимает OTLP/HTTP.
// Здесь: логи zerolog в stderr (по умолчанию тихо — WARN, PLAIN), метрики в
// никуда (CLI некому скрейпить), трассы — OTLP/HTTP в шлюз платформы.
// Возвращённый flush досылает накопленные спаны; вызывать при завершении.
func newObserver(version string) (observability.Observer, func(context.Context) error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	observability.SetPropagator(otelPropagator{})

	logger := observabilityzerolog.NewLogger(observabilityzerolog.Config{
		Level:    logLevelFromEnv(),
		Encoding: logEncodingFromEnv(),
	})

	tracer, flush := newTracer(logger, version)

	obs := observability.New(obsServiceName, obsEnvironment,
		logger, nopMetrics{}, observabilityotel.New(tracer),
		observability.WithTraceContextExtractor(traceContextExtractor),
		observability.WithAppVersion(version),
	)
	return obs, flush
}

// newTracer создаёт OTel-трейсер с экспортом в OTLP/HTTP-шлюз платформы.
// Без сети команда не должна замедляться: ретраи выключены, а время на
// экспорт ограничивает контекст flush у вызывающего.
func newTracer(logger observability.Logger, version string) (trace.Tracer, func(context.Context) error) {
	noFlush := func(context.Context) error { return nil }
	endpoint := envOr(envOTLPURL, defaultOTLPURL)
	if endpoint == otlpURLOff {
		return tracenoop.NewTracerProvider().Tracer(obsServiceName), noFlush
	}

	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpointURL(endpoint),
		otlptracehttp.WithRetry(otlptracehttp.RetryConfig{Enabled: false}),
	)
	if err != nil {
		logger.Warn(context.Background(), "трассировка выключена: некорректный "+envOTLPURL,
			observability.String("endpoint", endpoint), observability.Err(err))
		return tracenoop.NewTracerProvider().Tracer(obsServiceName), noFlush
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewSchemaless(
			attribute.String("service.name", obsServiceName),
			attribute.String("service.version", version),
			attribute.String("deployment.environment", obsEnvironment),
		)),
	)
	return tp.Tracer(obsServiceName), tp.Shutdown
}

// instrumentCommands оборачивает Action листовых команд корневым спаном трассы:
// имя — путь команды («protocols sync»), ошибка команды попадает в спан, а
// HTTP-запросы к платформе становятся его дочерними спанами.
func instrumentCommands(obs observability.Observer, cmds []*cli.Command, prefix string) {
	for _, cmd := range cmds {
		name := strings.TrimSpace(prefix + " " + cmd.Name)
		instrumentCommands(obs, cmd.Commands, name)
		if cmd.Action == nil {
			continue
		}
		action := cmd.Action
		cmd.Action = func(ctx context.Context, c *cli.Command) error {
			ctx, span := obs.Start(ctx, name)
			defer span.End()
			err := action(ctx, c)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(observability.SpanStatusError, err.Error())
			}
			return err
		}
	}
}

// otelPropagator адаптирует глобальный OTel TextMapPropagator к
// observability.Propagator (как в observabilitydefault, где адаптер приватный).
type otelPropagator struct{}

func (otelPropagator) Extract(ctx context.Context, carrier observability.TextMapCarrier) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

func (otelPropagator) Inject(ctx context.Context, carrier observability.TextMapCarrier) {
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

func traceContextExtractor(ctx context.Context) (string, string) {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if !sc.IsValid() {
		return "", ""
	}
	return sc.TraceID().String(), sc.SpanID().String()
}

// logLevelFromEnv — уровень из OBSERVABILITY_LOG_LEVEL; по умолчанию WARN,
// чтобы диагностические логи не мешали выводу команд.
func logLevelFromEnv() observability.Level {
	level := observability.LevelWarn
	if raw := os.Getenv(envLogLevel); raw != "" {
		_ = level.UnmarshalText([]byte(raw))
	}
	return level
}

// logEncodingFromEnv — формат из OBSERVABILITY_LOG_ENCODING; по умолчанию
// PLAIN — логи CLI читает человек в терминале, а не сборщик.
func logEncodingFromEnv() observability.LogEncoding {
	encoding := observability.LogEncodingPlain
	if raw := os.Getenv(envLogEncoding); raw != "" {
		_ = encoding.UnmarshalText([]byte(raw))
	}
	return encoding
}

// nopMetrics — заглушка Metrics: метрики короткоживущего CLI некому отдавать
// (pull-модель), а транспортным модулям SDK реализация обязательна.
type nopMetrics struct{}

func (nopMetrics) Counter(string, ...observability.MetricOption) observability.Counter {
	return nopInstrument{}
}

func (nopMetrics) Gauge(string, ...observability.MetricOption) observability.Gauge {
	return nopInstrument{}
}

func (nopMetrics) Histogram(string, ...observability.MetricOption) observability.Histogram {
	return nopInstrument{}
}

type nopInstrument struct{}

func (nopInstrument) Inc(context.Context, ...observability.KeyValue)              {}
func (nopInstrument) Add(context.Context, float64, ...observability.KeyValue)     {}
func (nopInstrument) Set(context.Context, float64, ...observability.KeyValue)     {}
func (nopInstrument) Observe(context.Context, float64, ...observability.KeyValue) {}
