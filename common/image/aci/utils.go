package aci

import (
	"fmt"
	"sort"
	"strings"

	"github.com/appc/spec/schema/types"
)

func getLabelPriority(name types.ACIdentifier) int {
	labelsPriority := map[types.ACIdentifier]int{
		"version": 0,
		"os":      1,
		"arch":    2,
	}
	if i, ok := labelsPriority[name]; ok {
		return i
	}
	return len(labelsPriority) + 1
}

// labelsSlice implements sort.Interface for types.Labels
type labelsSlice types.Labels

func (p labelsSlice) Len() int { return len(p) }
func (p labelsSlice) Less(i, j int) bool {
	return getLabelPriority(p[i].Name) < getLabelPriority(p[j].Name)
}
func (p labelsSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

func labelsToString(inLabels types.Labels) string {
	// take a copy to avoid changing the original slice
	labels := append(types.Labels(nil), inLabels...)
	sort.Sort(labelsSlice(labels))

	var out []string
	for _, l := range labels {
		out = append(out, fmt.Sprintf("%q:%q", l.Name, l.Value))
	}
	return "[" + strings.Join(out, ", ") + "]"
}
