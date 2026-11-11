package databaseaccessk8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/TraumTech/paas-cli/internal/entities"
)

func cnpgCluster(namespace, name string, roles ...any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1", "kind": "Cluster",
		"metadata": map[string]any{"name": name, "namespace": namespace},
		"spec":     map[string]any{"managed": map[string]any{"roles": roles}},
	}}
}

func cnpgDatabase(namespace, name, cluster, db string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1", "kind": "Database",
		"metadata": map[string]any{"name": name, "namespace": namespace},
		"spec":     map[string]any{"cluster": map[string]any{"name": cluster}, "name": db},
	}}
}

func fakeCluster(objects ...runtime.Object) *cluster {
	lists := map[schema.GroupVersionResource]string{clustersGVR: "ClusterList", databasesGVR: "DatabaseList"}
	return &cluster{dyn: dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), lists, objects...)}
}

func TestFind_ReadsRolesAndDatabases(t *testing.T) {
	c := fakeCluster(
		cnpgCluster("d0123", "main", map[string]any{"name": "nomadvisa"}),
		cnpgDatabase("d0123", "nomadvisa", "main", "nomadvisa"),
		cnpgDatabase("d0123", "other", "second", "other"),
	)

	server, err := c.Find(context.Background(), "main", "")

	require.NoError(t, err)
	assert.Equal(t, &entities.DatabaseServer{Namespace: "d0123", Name: "main", Roles: []string{"nomadvisa"}, Databases: []string{"nomadvisa"}}, server)
}

func TestFind_SameNameInTwoOrganizations(t *testing.T) {
	c := fakeCluster(cnpgCluster("d0123", "main"), cnpgCluster("d4567", "main"))

	_, err := c.Find(context.Background(), "main", "")

	var ambiguous *entities.AmbiguousDatabaseServerError
	require.ErrorAs(t, err, &ambiguous)
	assert.Equal(t, []string{"d0123", "d4567"}, ambiguous.Namespaces)

	server, err := c.Find(context.Background(), "main", "d4567")
	require.NoError(t, err)
	assert.Equal(t, "d4567", server.Namespace)
}

func TestFind_Missing(t *testing.T) {
	_, err := fakeCluster().Find(context.Background(), "main", "")
	require.ErrorIs(t, err, entities.ErrDatabaseServerNotFound)
}

func TestWithPersonalRole_KeepsPlatformRoles(t *testing.T) {
	platform := map[string]any{"name": "nomadvisa", "login": true}
	stale := map[string]any{"name": "ruslan", "login": false}

	roles := withPersonalRole([]any{platform, stale}, "ruslan", "paas-user-ruslan")

	require.Len(t, roles, 2)
	assert.Equal(t, platform, roles[0])
	personal := roles[1].(map[string]any)
	assert.Equal(t, "ruslan", personal["name"])
	assert.Equal(t, true, personal["login"])
	assert.Equal(t, personalRoleGrants, personal["inRoles"])
	assert.Equal(t, map[string]any{"name": "paas-user-ruslan"}, personal["passwordSecret"])
}

func TestRoleState(t *testing.T) {
	obj := unstructured.Unstructured{Object: map[string]any{"status": map[string]any{"managedRolesStatus": map[string]any{
		"byStatus":        map[string]any{"reconciled": []any{"nomadvisa", "ruslan"}},
		"cannotReconcile": map[string]any{"broken": []any{"role \"x\" does not exist"}},
	}}}}

	ready, _ := roleState(obj, "ruslan")
	assert.True(t, ready)
	ready, reason := roleState(obj, "broken")
	assert.False(t, ready)
	assert.Equal(t, `role "x" does not exist`, reason)
	ready, reason = roleState(obj, "pending")
	assert.False(t, ready)
	assert.Empty(t, reason)
}

func TestSecretName(t *testing.T) {
	assert.Equal(t, "paas-user-ruslan-ro", secretName("ruslan_ro"))
}
