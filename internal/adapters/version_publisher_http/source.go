package versionpublisherhttp

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/google/uuid"

	"github.com/TraumTech/paas-cli/internal/adapters/platformhttp"
	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/pkg/platformapi"
)

// Source фиксирует версию сервиса в API платформы через сгенерированный из
// контракта клиент (pkg/platformapi). Платформа идемпотентна: одна ревизия — одна
// версия, повторный вызов возвращает ту же версию.
type Source struct {
	client *platformapi.ClientWithResponses
}

func New(baseURL string, httpClient *http.Client) (*Source, error) {
	client, err := platformapi.NewClientWithResponses(baseURL, platformapi.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("build platform client: %w", err)
	}
	return &Source{client: client}, nil
}

func (s *Source) PublishVersion(ctx context.Context, serviceID, environment, commitRevision, branch, image string, form *entities.VersionForm) (*entities.Version, error) {
	id, err := uuid.Parse(serviceID)
	if err != nil {
		return nil, fmt.Errorf("неверный id сервиса %q: %w", serviceID, err)
	}

	body := platformapi.PublishVersionJSONRequestBody{CommitRevision: commitRevision}
	if environment != "" {
		env := platformapi.PublishVersionInputBodyEnvironment(environment)
		body.Environment = &env
	}
	if branch != "" {
		body.Branch = &branch
	}
	if image != "" {
		body.Image = &image
	}
	body.Form = formToAPI(form)
	resp, err := s.client.PublishVersionWithResponse(ctx, id, body)
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	// 201 — версия создана, 200 — уже существовала (идемпотентный повтор той же
	// ревизии, штатно при перезапуске выкатки); тело в обоих случаях одно.
	var version *platformapi.VersionResponse
	switch resp.StatusCode() {
	case http.StatusCreated:
		version = resp.JSON201
	case http.StatusOK:
		version = resp.JSON200
	case http.StatusNotFound:
		return nil, entities.ErrServiceNotFound
	default:
		return nil, fmt.Errorf("платформа ответила %s", resp.Status())
	}
	if version == nil {
		return nil, fmt.Errorf("платформа вернула пустой ответ")
	}
	return mapVersion(version), nil
}

// formToAPI: nil остаётся nil — «формы нет» не превращается в пустую форму.
func formToAPI(form *entities.VersionForm) *platformapi.VersionFormBody {
	if form == nil {
		return nil
	}
	out := &platformapi.VersionFormBody{Processes: make([]platformapi.ProcessFormBody, 0, len(form.Processes))}
	// Значения уже разрешены под окружение версии (DEP-14/15): платформа
	// получает готовый набор, а не правило слияния.
	if len(form.Variables) > 0 {
		variables := make([]platformapi.FormVariableBody, 0, len(form.Variables))
		for _, v := range form.Variables {
			variables = append(variables, platformapi.FormVariableBody{Name: v.Name, Value: v.Value})
		}
		out.Variables = &variables
	}
	if form.Replicas != 0 {
		replicas := int64(form.Replicas)
		out.Replicas = &replicas
	}
	for _, p := range form.Processes {
		body := platformapi.ProcessFormBody{Name: p.Name}
		if p.Listen != 0 {
			port := int64(p.Listen)
			body.ListenPort = &port
		}
		if len(p.Command) > 0 {
			command := p.Command
			body.Command = &command
		}
		if p.CPU != "" {
			cpu := p.CPU
			body.Cpu = &cpu
		}
		if p.Memory != "" {
			memory := p.Memory
			body.Memory = &memory
		}
		// Маршрут (DEP-10) едет с версией; префикс без зоны платформа отклонит.
		if p.Zone != "" {
			zone := p.Zone
			body.Zone = &zone
		}
		if p.Prefix != "" {
			prefix := p.Prefix
			body.Prefix = &prefix
		}
		out.Processes = append(out.Processes, body)
	}
	return out
}

func mapVersion(v *platformapi.VersionResponse) *entities.Version {
	return &entities.Version{
		ID:             v.Id.String(),
		Environment:    string(v.Environment),
		Number:         int(v.Number),
		CommitRevision: v.CommitRevision,
		Branch:         derefString(v.Branch),
		CreatedAt:      v.CreatedAt,
	}
}

// derefString — необязательное поле контракта: ветки может не быть (DEP-17).
func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// PublishBuild фиксирует сборку ветки (DEP-18): окружения в запросе нет, форма
// едет со всеми секциями — сливает их выкатка.
func (s *Source) PublishBuild(ctx context.Context, serviceID string, in entities.BuildRequest) (*entities.Build, error) {
	id, err := uuid.Parse(serviceID)
	if err != nil {
		return nil, fmt.Errorf("неверный id сервиса %q: %w", serviceID, err)
	}

	body := platformapi.PublishBuildJSONRequestBody{CommitRevision: in.CommitRevision}
	if in.Branch != "" {
		body.Branch = &in.Branch
	}
	if in.Image != "" {
		body.Image = &in.Image
	}
	if in.Contract != "" {
		body.Contract = &in.Contract
		format := platformapi.PublishBuildInputBodyContractFormat(in.ContractFormat)
		body.ContractFormat = &format
	}
	body.Form = buildFormToAPI(in.Form)

	resp, err := s.client.PublishBuildWithResponse(ctx, id, body)
	if err != nil {
		return nil, platformhttp.RequestError(err)
	}
	// 201 — сборка заведена, 200 — эта ревизия уже собрана (штатный повтор).
	var build *platformapi.BuildResponse
	switch resp.StatusCode() {
	case http.StatusCreated:
		build = resp.JSON201
	case http.StatusOK:
		build = resp.JSON200
	case http.StatusNotFound:
		return nil, entities.ErrServiceNotFound
	default:
		return nil, fmt.Errorf("платформа ответила %s", resp.Status())
	}
	if build == nil {
		return nil, fmt.Errorf("платформа вернула пустой ответ")
	}
	return &entities.Build{
		ID:             build.Id.String(),
		CommitRevision: build.CommitRevision,
		Branch:         derefString(build.Branch),
		CreatedAt:      build.CreatedAt,
	}, nil
}

// buildFormToAPI переносит объявление формы как есть: секции окружений едут
// неразрешёнными, потому что окружение выбирает выкатка.
func buildFormToAPI(form *entities.FormDeclaration) *platformapi.BuildFormBody {
	if form == nil {
		return nil
	}
	out := &platformapi.BuildFormBody{Processes: []platformapi.ProcessFormBody{}}
	for _, p := range form.Processes {
		process := platformapi.ProcessFormBody{Name: p.Name}
		if p.Listen != 0 {
			port := int64(p.Listen)
			process.ListenPort = &port
		}
		if len(p.Command) > 0 {
			command := p.Command
			process.Command = &command
		}
		if p.CPU != "" {
			cpu := p.CPU
			process.Cpu = &cpu
		}
		if p.Memory != "" {
			memory := p.Memory
			process.Memory = &memory
		}
		if p.Zone != "" {
			zone := p.Zone
			process.Zone = &zone
		}
		if p.Prefix != "" {
			prefix := p.Prefix
			process.Prefix = &prefix
		}
		out.Processes = append(out.Processes, process)
	}

	// Базы (DB-03) — в порядке объявления: он и так детерминирован файлом.
	if len(form.Databases) > 0 {
		databases := make([]platformapi.DatabaseFormBody, 0, len(form.Databases))
		for _, d := range form.Databases {
			database := platformapi.DatabaseFormBody{
				Name:   d.Name,
				Engine: platformapi.DatabaseFormBodyEngine(d.Engine),
				Server: d.Server,
			}
			if d.Variable != "" {
				variable := d.Variable
				database.Variable = &variable
			}
			databases = append(databases, database)
		}
		out.Databases = &databases
	}

	// Порядок секций детерминирован: одна и та же ревизия не должна публиковать
	// форму по-разному от запуска к запуску.
	names := make([]string, 0, len(form.Environments))
	for name := range form.Environments {
		names = append(names, name)
	}
	sort.Strings(names)
	environments := make([]platformapi.FormEnvironmentBody, 0, len(names))
	for _, name := range names {
		values := form.Environments[name]
		section := platformapi.FormEnvironmentBody{Name: name}
		if values.Replicas != 0 {
			replicas := int64(values.Replicas)
			section.Replicas = &replicas
		}
		variables := make([]platformapi.FormVariableBody, 0, len(values.Variables))
		for _, varName := range sortedKeys(values.Variables) {
			variables = append(variables, platformapi.FormVariableBody{Name: varName, Value: values.Variables[varName]})
		}
		if len(variables) > 0 {
			section.Variables = &variables
		}
		if len(values.Databases) > 0 {
			overrides := make([]platformapi.DatabaseOverrideBody, 0, len(values.Databases))
			for _, database := range sortedOverrideKeys(values.Databases) {
				overrides = append(overrides, platformapi.DatabaseOverrideBody{Name: database, Server: values.Databases[database].Server})
			}
			section.Databases = &overrides
		}
		environments = append(environments, section)
	}
	if len(environments) > 0 {
		out.Environments = &environments
	}
	return out
}

func sortedKeys(values map[string]string) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedOverrideKeys(overrides map[string]entities.DatabaseOverride) []string {
	names := make([]string, 0, len(overrides))
	for name := range overrides {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
