package entities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/TraumTech/paas-cli/internal/entities"
)

func TestManifestEffectiveDestination(t *testing.T) {
	assert.Equal(t, "protocols", (&entities.Manifest{}).EffectiveDestination())
	assert.Equal(t, "protocols", (&entities.Manifest{Destination: "  "}).EffectiveDestination())
	assert.Equal(t, "vendor/api", (&entities.Manifest{Destination: "vendor/api"}).EffectiveDestination())
}

func TestManifestValidate_OK(t *testing.T) {
	m := &entities.Manifest{Dependencies: []entities.ManifestDependency{
		{Name: "paas-backend"},
		{Name: "billing", Methods: []string{"op-a"}},
	}}
	assert.NoError(t, m.Validate())
}

func TestManifestValidate_NoDependencies(t *testing.T) {
	assert.ErrorIs(t, (&entities.Manifest{}).Validate(), entities.ErrManifestNoDependencies)
}

func TestManifestValidate_EmptyName(t *testing.T) {
	m := &entities.Manifest{Dependencies: []entities.ManifestDependency{{Name: "  "}}}
	assert.ErrorIs(t, m.Validate(), entities.ErrManifestDependencyNoName)
}

func TestManifestValidate_Duplicate(t *testing.T) {
	m := &entities.Manifest{Dependencies: []entities.ManifestDependency{
		{Name: "paas-backend"},
		{Name: "paas-backend"},
	}}
	var dup *entities.ManifestDuplicateError
	assert.ErrorAs(t, m.Validate(), &dup)
	assert.Equal(t, "paas-backend", dup.Name)
}
