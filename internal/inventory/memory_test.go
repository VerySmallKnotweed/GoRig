package inventory

import (
	"GoRig/internal/report"
	"testing"
)

func TestSumModules(t *testing.T) {
	module := func(value, status string) report.Component {
		return report.Component{Attributes: map[string]report.Value{"capacity_bytes": {Value: value, Status: status}}}
	}
	if v := sumModules([]report.Component{module("8589934592", "collected"), module("4294967296", "collected")}); v.Value != "12884901888" || v.Status != "collected" {
		t.Fatalf("sum: %+v", v)
	}
	for _, modules := range [][]report.Component{nil, {module("8589934592", "collected"), module("", "unavailable")}, {module("18446744073709551615", "collected"), module("1", "collected")}} {
		if v := sumModules(modules); v.Status != "unavailable" {
			t.Fatalf("partial sum: %+v", v)
		}
	}
}
