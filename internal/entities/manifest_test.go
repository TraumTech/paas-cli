package entities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/entities"
)

func TestManifestEffectiveDestination(t *testing.T) {
	assert.Equal(t, "protocols", (&entities.Manifest{}).EffectiveDestination())
	assert.Equal(t, "protocols", (&entities.Manifest{Destination: "  "}).EffectiveDestination())
	assert.Equal(t, "vendor/api", (&entities.Manifest{Destination: "vendor/api"}).EffectiveDestination())
}

func ownService() *entities.ManifestService {
	return &entities.ManifestService{Name: "frontend"}
}

func TestManifestValidate_OK(t *testing.T) {
	m := &entities.Manifest{Service: ownService(), Dependencies: []entities.ManifestDependency{
		{Name: "paas-backend"},
		{Name: "billing", Methods: []string{"op-a"}},
	}}
	assert.NoError(t, m.Validate())
}

func TestManifestValidate_NoService(t *testing.T) {
	m := &entities.Manifest{Dependencies: []entities.ManifestDependency{{Name: "paas-backend"}}}
	assert.ErrorIs(t, m.Validate(), entities.ErrManifestNoService)
}

func TestManifestValidate_ServiceNoName(t *testing.T) {
	m := &entities.Manifest{Service: &entities.ManifestService{Name: " "}, Dependencies: []entities.ManifestDependency{{Name: "paas-backend"}}}
	assert.ErrorIs(t, m.Validate(), entities.ErrManifestServiceNoName)
}

func TestManifestValidate_NoDependencies(t *testing.T) {
	assert.ErrorIs(t, (&entities.Manifest{Service: ownService()}).Validate(), entities.ErrManifestNoDependencies)
}

func TestManifestValidate_EmptyName(t *testing.T) {
	m := &entities.Manifest{Service: ownService(), Dependencies: []entities.ManifestDependency{{Name: "  "}}}
	assert.ErrorIs(t, m.Validate(), entities.ErrManifestDependencyNoName)
}

// Прежняя форма — contract в [service] — разворачивается в единственную запись
// без имени (протокол по умолчанию): существующие манифесты публикуют то же,
// что раньше (CLI-23).
func TestManifestRequireProtocols_LegacyContract(t *testing.T) {
	m := &entities.Manifest{Service: &entities.ManifestService{Name: "paas-backend", Contract: "openapi.json", Format: "openapi"}}
	protocols, err := m.RequireProtocols()
	require.NoError(t, err)
	require.Len(t, protocols, 1)
	assert.Equal(t, entities.ManifestProtocol{Name: "", Contract: "openapi.json", Format: "openapi"}, protocols[0])
}

func TestManifestRequireProtocols_List(t *testing.T) {
	m := &entities.Manifest{
		Service: &entities.ManifestService{Name: "paas-backend"},
		Protocols: []entities.ManifestProtocol{
			{Name: "http", Contract: "openapi.json"},
			{Name: "grpc", Contract: "api/edge.proto", Format: "grpc"},
		},
	}
	protocols, err := m.RequireProtocols()
	require.NoError(t, err)
	assert.Equal(t, m.Protocols, protocols)
}

func TestManifestRequireProtocols_Missing(t *testing.T) {
	_, err := (&entities.Manifest{}).RequireProtocols()
	assert.ErrorIs(t, err, entities.ErrManifestNoService)
}

func TestManifestRequireProtocols_NoName(t *testing.T) {
	m := &entities.Manifest{Service: &entities.ManifestService{Name: "  ", Contract: "openapi.json"}}
	_, err := m.RequireProtocols()
	assert.ErrorIs(t, err, entities.ErrManifestServiceNoName)
}

func TestManifestRequireProtocols_NoContract(t *testing.T) {
	m := &entities.Manifest{Service: &entities.ManifestService{Name: "paas-backend", Contract: " "}}
	_, err := m.RequireProtocols()
	assert.ErrorIs(t, err, entities.ErrManifestServiceNoContract)
}

// Контракт объявлен и старой формой, и перечнем — неоднозначность, а не
// молчаливый приоритет одной из форм.
func TestManifestDeclaredProtocols_MixedForms(t *testing.T) {
	m := &entities.Manifest{
		Service:   &entities.ManifestService{Name: "paas-backend", Contract: "openapi.json"},
		Protocols: []entities.ManifestProtocol{{Name: "http", Contract: "openapi.json"}},
	}
	_, err := m.DeclaredProtocols()
	assert.ErrorIs(t, err, entities.ErrManifestMixedContractForms)
}

func TestManifestDeclaredProtocols_EntryNoName(t *testing.T) {
	m := &entities.Manifest{Protocols: []entities.ManifestProtocol{{Contract: "openapi.json"}}}
	_, err := m.DeclaredProtocols()
	assert.ErrorIs(t, err, entities.ErrManifestProtocolNoName)
}

func TestManifestDeclaredProtocols_EntryNoContract(t *testing.T) {
	m := &entities.Manifest{Protocols: []entities.ManifestProtocol{{Name: "http"}}}
	_, err := m.DeclaredProtocols()
	var noContract *entities.ManifestProtocolNoContractError
	require.ErrorAs(t, err, &noContract)
	assert.Equal(t, "http", noContract.Name)
}

func TestManifestDeclaredProtocols_DuplicateName(t *testing.T) {
	m := &entities.Manifest{Protocols: []entities.ManifestProtocol{
		{Name: "http", Contract: "a.json"},
		{Name: "http", Contract: "b.json"},
	}}
	_, err := m.DeclaredProtocols()
	var dup *entities.ManifestDuplicateProtocolError
	require.ErrorAs(t, err, &dup)
	assert.Equal(t, "http", dup.Name)
}

// Контракта нет ни в одной форме — пустой перечень без ошибки: он есть не у
// каждого репозитория (чистый потребитель).
func TestManifestDeclaredProtocols_Empty(t *testing.T) {
	m := &entities.Manifest{Service: &entities.ManifestService{Name: "paas-cli"}}
	protocols, err := m.DeclaredProtocols()
	require.NoError(t, err)
	assert.Empty(t, protocols)
}

func TestManifestValidate_AttributesWithoutMethods(t *testing.T) {
	// Атрибут живёт внутри объявленного метода (PRT-29): attributes без methods —
	// понятная ошибка с именем зависимости.
	m := &entities.Manifest{Service: ownService(), Dependencies: []entities.ManifestDependency{
		{Name: "paas-backend", Attributes: []string{"GET /services#response.200.name"}},
	}}
	var noMethods *entities.ManifestAttributesWithoutMethodsError
	assert.ErrorAs(t, m.Validate(), &noMethods)
	assert.Equal(t, "paas-backend", noMethods.Name)
}

func TestManifestValidate_AttributesWithMethods(t *testing.T) {
	m := &entities.Manifest{Service: ownService(), Dependencies: []entities.ManifestDependency{
		{Name: "paas-backend", Methods: []string{"GET /services"}, Attributes: []string{"GET /services#response.200.name"}},
	}}
	assert.NoError(t, m.Validate())
}

func TestManifestValidate_Duplicate(t *testing.T) {
	m := &entities.Manifest{Service: ownService(), Dependencies: []entities.ManifestDependency{
		{Name: "paas-backend"},
		{Name: "paas-backend"},
	}}
	var dup *entities.ManifestDuplicateError
	assert.ErrorAs(t, m.Validate(), &dup)
	assert.Equal(t, "paas-backend", dup.Name)
}
