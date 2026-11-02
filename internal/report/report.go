package report

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const SchemaVersion = 1

type Report struct {
	SchemaVersion    int       `json:"schema_version"`
	ScanID           string    `json:"scan_id"`
	CollectedAt      time.Time `json:"collected_at"`
	CollectorVersion string    `json:"collector_version"`
	Device           Device    `json:"device"`
}

type Device struct {
	ComputerName Value       `json:"computer_name"`
	Manufacturer Value       `json:"manufacturer"`
	Model        Value       `json:"model"`
	SerialNumber Value       `json:"serial_number"`
	OS           Value       `json:"os"`
	Components   []Component `json:"components"`
}

type Component struct {
	Type         string `json:"type"`
	Name         Value  `json:"name"`
	Manufacturer Value  `json:"manufacturer"`
	Model        Value  `json:"model"`
	SerialNumber Value  `json:"serial_number"`
}

type Value struct {
	Value        string        `json:"value"`
	Raw          string        `json:"raw,omitempty"`
	Source       string        `json:"source,omitempty"`
	Status       string        `json:"status"`
	Observations []Observation `json:"observations,omitempty"`
}

type Observation struct {
	Value  string `json:"value,omitempty"`
	Raw    string `json:"raw,omitempty"`
	Source string `json:"source"`
	Status string `json:"status"`
}

func New(collectorVersion string) (Report, error) {
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return Report{}, err
	}

	return Report{
		SchemaVersion:    SchemaVersion,
		ScanID:           hex.EncodeToString(idBytes),
		CollectedAt:      time.Now().UTC(),
		CollectorVersion: collectorVersion,
		Device: Device{
			Components: []Component{},
		},
	}, nil
}
