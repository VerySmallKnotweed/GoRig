//go:build windows

package inventory

import (
	"testing"

	"GoRig/internal/report"
)

func TestSelectPrimaryValue(t *testing.T) {
	tests := []struct {
		name         string
		observations []report.Observation
		wantValue    string
		wantStatus   string
		wantSource   string
	}{
		{
			name: "обычное значение важнее шаблонного",
			observations: []report.Observation{
				{
					Value:  "Default string",
					Source: "WMI",
					Status: "template",
				},
				{
					Value:  "Example Maker",
					Source: "Registry",
					Status: "collected",
				},
			},
			wantValue:  "Example Maker",
			wantStatus: "collected",
			wantSource: "Registry",
		},
		{
			name: "сохраняет шаблон, если других значений нет",
			observations: []report.Observation{
				{
					Value:  "Default string",
					Source: "WMI",
					Status: "template",
				},
			},
			wantValue:  "Default string",
			wantStatus: "template",
			wantSource: "WMI",
		},
		{
			name: "нет доступных значений",
			observations: []report.Observation{
				{
					Source: "Registry",
					Status: "unavailable",
				},
			},
			wantValue:  "",
			wantStatus: "unavailable",
			wantSource: "Registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectPrimaryValue(tt.observations)

			if got.Value != tt.wantValue ||
				got.Status != tt.wantStatus ||
				got.Source != tt.wantSource {
				t.Errorf(
					"получено Value=%q Status=%q Source=%q; ожидалось Value=%q Status=%q Source=%q",
					got.Value, got.Status, got.Source,
					tt.wantValue, tt.wantStatus, tt.wantSource,
				)
			}

			if len(got.Observations) != len(tt.observations) {
				t.Errorf(
					"сохранено наблюдений: %d; ожидалось: %d",
					len(got.Observations), len(tt.observations),
				)
			}
		})
	}
}

func TestMakeObservationMarksTemplate(t *testing.T) {
	got := makeObservation("Default string", "WMI")

	if got.Status != "template" {
		t.Errorf("статус = %q; ожидался template", got.Status)
	}
	if got.Value != "Default string" || got.Raw != "Default string" {
		t.Errorf("исходное значение изменилось: %+v", got)
	}
}
