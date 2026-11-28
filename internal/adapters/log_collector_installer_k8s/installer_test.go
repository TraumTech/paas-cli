package logcollectorinstallerk8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/TraumTech/paas-cli/internal/entities"
)

var collector = entities.LogCollector{PushURL: entities.DefaultLogPushURL, Key: "s3cret"}

// TRA-559/AC-4: в кластер попадает сборщик со своим ключом — и ничего сверх.
func TestObjects(t *testing.T) {
	objects, err := New().Objects(collector)
	require.NoError(t, err)
	assert.Equal(t, []entities.ManifestObject{
		{Kind: "Namespace", Name: "stapel-logs"},
		{Kind: "ServiceAccount", Namespace: "stapel-logs", Name: "collector"},
		{Kind: "ClusterRole", Name: "stapel-logs-collector"},
		{Kind: "ClusterRoleBinding", Name: "stapel-logs-collector"},
		{Kind: "Secret", Namespace: "stapel-logs", Name: "collector"},
		{Kind: "ConfigMap", Namespace: "stapel-logs", Name: "collector"},
		{Kind: "DaemonSet", Namespace: "stapel-logs", Name: "collector"},
	}, objects)
}

// Ключ лежит только в секрете; его смена меняет шаблон пода — сборщик
// перекатится и подхватит новый.
func TestRenderKeyAndRollout(t *testing.T) {
	first, err := render(collector)
	require.NoError(t, err)
	assert.Contains(t, first, "key: czNjcmV0")
	assert.NotContains(t, first, "s3cret")
	assert.Contains(t, first, `value: "https://logs.stapel.sh/loki/api/v1/push"`)

	again, err := render(collector)
	require.NoError(t, err)
	assert.Equal(t, first, again)

	rotated, err := render(entities.LogCollector{PushURL: collector.PushURL, Key: "other"})
	require.NoError(t, err)
	assert.NotEqual(t, first, rotated)
}

func TestDaemonSetReady(t *testing.T) {
	ds := &appsv1.DaemonSet{}
	assert.False(t, daemonSetReady(ds), "нод ещё нет")
	ds.Status = appsv1.DaemonSetStatus{DesiredNumberScheduled: 2, UpdatedNumberScheduled: 2, NumberAvailable: 1}
	assert.False(t, daemonSetReady(ds))
	ds.Status.NumberAvailable = 2
	assert.True(t, daemonSetReady(ds))
}
