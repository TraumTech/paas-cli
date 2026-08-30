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
	assert.Equal(t, platformapi.DatabaseFormBody{Name: "main", Engine: platformapi.Postgres, Server: "paas-postgres"}, databases[0])
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

func TestBuildFormToAPI_WithoutDatabases(t *testing.T) {
	body := buildFormToAPI(&entities.FormDeclaration{Processes: []entities.ProcessForm{{Name: "server"}}})

	assert.Nil(t, body.Databases)
	assert.Nil(t, body.Environments)
}
