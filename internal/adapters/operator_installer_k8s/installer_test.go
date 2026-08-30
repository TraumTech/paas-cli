package operatorinstallerk8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/entities"
)

const manifest = `apiVersion: v1
kind: Namespace
metadata:
  name: cnpg-system
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: clusters.postgresql.cnpg.io
---

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cnpg-controller-manager
  namespace: cnpg-system
`

// Пустые документы пропускаются; порядок манифеста сохраняется.
func TestObjects(t *testing.T) {
	objects, err := New().Objects(manifest)
	require.NoError(t, err)
	assert.Equal(t, []entities.ManifestObject{
		{Kind: "Namespace", Name: "cnpg-system"},
		{Kind: "CustomResourceDefinition", Name: "clusters.postgresql.cnpg.io"},
		{Kind: "Deployment", Namespace: "cnpg-system", Name: "cnpg-controller-manager"},
	}, objects)
}

// CRD уходят вперёд, остальное — в порядке манифеста (namespace раньше того,
// что в нём лежит).
func TestSplitPutsCRDsFirst(t *testing.T) {
	objects, err := parse(manifest)
	require.NoError(t, err)
	crds, rest := split(objects)
	require.Len(t, crds, 1)
	assert.Equal(t, "clusters.postgresql.cnpg.io", crds[0].GetName())
	require.Len(t, rest, 2)
	assert.Equal(t, "Namespace", rest[0].GetKind())
}

func TestParseRejectsBrokenManifest(t *testing.T) {
	for name, input := range map[string]string{
		"пусто":     "",
		"без kind":  "metadata:\n  name: x\n",
		"не YAML":   "{{{",
		"без имени": "kind: Namespace\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parse(input)
			assert.ErrorIs(t, err, entities.ErrOperatorManifestBroken)
		})
	}
}
