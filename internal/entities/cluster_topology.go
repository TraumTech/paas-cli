package entities

import "sort"

// ClusterTopology — заявление владельца о зонах кластера (CLS-03): zonal —
// ноды в одной зоне, regional — обещание нод минимум в двух зонах. Команда
// лишь предлагает значение по меткам нод; решает владелец.
type ClusterTopology string

const (
	ClusterTopologyZonal    ClusterTopology = "zonal"
	ClusterTopologyRegional ClusterTopology = "regional"
)

func ValidateClusterTopology(t ClusterTopology) error {
	switch t {
	case ClusterTopologyZonal, ClusterTopologyRegional:
		return nil
	default:
		return ErrUnknownClusterTopology
	}
}

// ProposeClusterTopology выводит предложение из зон, увиденных на нодах:
// две и более — региональный. Это подсказка, а не факт: ноды показывают
// момент, а не гарантию.
func ProposeClusterTopology(zones []string) ClusterTopology {
	if len(zones) >= 2 {
		return ClusterTopologyRegional
	}
	return ClusterTopologyZonal
}

// DistinctZones — уникальные непустые зоны в устойчивом порядке; ноды без
// метки зоны в счёт не идут.
func DistinctZones(labels []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, zone := range labels {
		if zone == "" {
			continue
		}
		if _, ok := seen[zone]; ok {
			continue
		}
		seen[zone] = struct{}{}
		out = append(out, zone)
	}
	sort.Strings(out)
	return out
}
