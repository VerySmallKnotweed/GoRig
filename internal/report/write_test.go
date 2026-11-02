package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteNewFileDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	first := Report{ScanID: "first"}
	if err := WriteNewFile(path, first); err != nil {
		t.Fatal(err)
	}
	if err := WriteNewFile(path, Report{ScanID: "second"}); err == nil {
		t.Fatal("existing report was overwritten")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result Report
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.ScanID != first.ScanID {
		t.Fatalf("scan ID = %q", result.ScanID)
	}
}

func TestWriteFileExplicitPathOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := WriteNewFile(path, Report{ScanID: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, Report{ScanID: "second"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result Report
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.ScanID != "second" {
		t.Fatalf("scan ID = %q", result.ScanID)
	}
}
