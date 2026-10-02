package versionpublisherhttp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/pkg/platformapi"
)

// Объявление баз (DB-03) уезжает как есть: переменная — только если объявлена,
// переопределения секций — в детерминированном порядке.
func TestBuildFormToAPI_Databases(t *testing.T) {
	form := &entities.FormDeclaration{
		Processes: []entities.ProcessForm{{Name: "server", Listen: 8080}},
		Databases: []entities.DatabaseForm{
			{Name: "main", Engine: "postgres", Server: "paas-postgres"},
			{Name: "reports", Engine: "postgres", Server: "paas-postgres", Variable: "REPORTS_DSN"},
		},
		Environments: map[string]entities.EnvironmentValues{
			"dev": {Databases: map[string]entities.DatabaseOverride{
				"reports": {Server: "dev-pg"},
				"main":    {Server: "dev-pg"},
			}},
		},
	}

	body := buildFormToAPI(form)

	require.NotNil(t, body.Databases)
	databases := *body.Databases
	require.Len(t, databases, 2)
	assert.Equal(t, platformapi.DatabaseFormBody{Name: "main", Engine: platformapi.DatabaseFormBodyEnginePostgres, Server: "paas-postgres"}, databases[0])
	require.NotNil(t, databases[1].Variable)
	assert.Equal(t, "REPORTS_DSN", *databases[1].Variable)

	require.NotNil(t, body.Environments)
	dev := (*body.Environments)[0]
	require.NotNil(t, dev.Databases)
	assert.Equal(t, []platformapi.DatabaseOverrideBody{
		{Name: "main", Server: "dev-pg"},
		{Name: "reports", Server: "dev-pg"},
	}, *dev.Databases)
}

// Объявление бакетов (OBJ-06): безымянный бакет остаётся с пустым именем,
// переопределение несёт только заданные поля.
func TestBuildFormToAPI_Buckets(t *testing.T) {
	form := &entities.FormDeclaration{
		Processes: []entities.ProcessForm{{Name: "server", Listen: 8080}},
		Buckets: []entities.BucketForm{
			{Server: "prod-s3", Size: "20Gi"},
			{Name: "media", Server: "prod-s3", Size: "5Gi"},
		},
		Environments: map[string]entities.EnvironmentValues{
			"dev": {Buckets: map[string]entities.BucketOverride{
				"media":   {Size: "1Gi"},
				"default": {Server: "dev-s3", Size: "1Gi"},
			}},
		},
	}

	body := buildFormToAPI(form)

	require.NotNil(t, body.Buckets)
	assert.Equal(t, []platformapi.BucketFormBody{
		{Server: "prod-s3", Size: "20Gi"},
		{Name: "media", Server: "prod-s3", Size: "5Gi"},
	}, *body.Buckets)

	require.NotNil(t, body.Environments)
	dev := (*body.Environments)[0]
	require.NotNil(t, dev.Buckets)
	overrides := *dev.Buckets
	require.Len(t, overrides, 2)
	assert.Equal(t, "default", overrides[0].Name)
	assert.Equal(t, "dev-s3", *overrides[0].Server)
	assert.Equal(t, "1Gi", *overrides[0].Size)
	assert.Equal(t, "media", overrides[1].Name)
	assert.Nil(t, overrides[1].Server)
	assert.Equal(t, "1Gi", *overrides[1].Size)
}

func TestBuildFormToAPI_WithoutDatabases(t *testing.T) {
	body := buildFormToAPI(&entities.FormDeclaration{Processes: []entities.ProcessForm{{Name: "server"}}})

	assert.Nil(t, body.Databases)
	assert.Nil(t, body.Buckets)
	assert.Nil(t, body.Environments)
}

// Внутренние именованные порты (DEP-24) едут как есть; без них поле не
// отправляется — старый edge принял бы лишнее поле за ошибку формы.
func TestBuildFormToAPI_Ports(t *testing.T) {
	body := buildFormToAPI(&entities.FormDeclaration{Processes: []entities.ProcessForm{
		{Name: "server", Listen: 8080, Ports: map[string]int{"admin": 4434, "metrics": 8003}},
		{Name: "worker"},
	}})

	require.NotNil(t, body.Processes[0].Ports)
	assert.Equal(t, map[string]int64{"admin": 4434, "metrics": 8003}, *body.Processes[0].Ports)
	assert.Nil(t, body.Processes[1].Ports)
}
