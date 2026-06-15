package entities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/TraumTech/paas-cli/internal/entities"
)

func TestVersionRequestValidate(t *testing.T) {
	assert.NoError(t, entities.VersionRequest{CommitRevision: "abc123"}.Validate())

	for _, rev := range []string{"", "   ", "\t\n"} {
		assert.ErrorIs(t, entities.VersionRequest{CommitRevision: rev}.Validate(), entities.ErrEmptyCommitRevision)
	}
}
