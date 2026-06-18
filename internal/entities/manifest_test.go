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

func TestManifestRequireService_OK(t *testing.T) {
	m := &entities.Manifest{Service: &entities.ManifestService{Name: "paas-backend", Contract: "openapi.json"}}
	svc, err := m.RequireService()
	require.NoError(t, err)
	assert.Equal(t, "paas-backend", svc.Name)
	assert.Equal(t, "openapi.json", svc.Contract)
}

func TestManifestRequireService_Missing(t *testing.T) {
	_, err := (&entities.Manifest{}).RequireService()
	assert.ErrorIs(t, err, entities.ErrManifestNoService)
}

func TestManifestRequireService_NoName(t *testing.T) {
	m := &entities.Manifest{Service: &entities.ManifestService{Name: "  ", Contract: "openapi.json"}}
	_, err := m.RequireService()
	assert.ErrorIs(t, err, entities.ErrManifestServiceNoName)
}

func TestManifestRequireService_NoContract(t *testing.T) {
	m := &entities.Manifest{Service: &entities.ManifestService{Name: "paas-backend", Contract: " "}}
	_, err := m.RequireService()
	assert.ErrorIs(t, err, entities.ErrManifestServiceNoContract)
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
