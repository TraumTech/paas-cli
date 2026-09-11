package clusterprovisionerk8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func node(zone string) corev1.Node {
	labels := map[string]string{}
	if zone != "" {
		labels[corev1.LabelTopologyZone] = zone
	}
	return corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: labels}}
}

// Зоны — уникальные, по стандартной метке, в устойчивом порядке; ноды без
// метки не считаются зоной.
func TestNodeZones(t *testing.T) {
	zones := nodeZones([]corev1.Node{node("ru-central1-b"), node(""), node("ru-central1-a"), node("ru-central1-b")})

	assert.Equal(t, []string{"ru-central1-a", "ru-central1-b"}, zones)
	assert.Nil(t, nodeZones([]corev1.Node{node("")}))
}
