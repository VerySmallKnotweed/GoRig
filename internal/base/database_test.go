package base

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"GoRig/internal/report"
)

func fixture(id string, at time.Time) report.Report {
	v := func(s string) report.Value { return report.Value{Value: s, Status: "collected", Source: "first"} }
	return report.Report{SchemaVersion: 1, ScanID: id, CollectedAt: at, Device: report.Device{
		ComputerName: v("PC"), Manufacturer: v("Maker"), Model: v("M"), SerialNumber: v("123"), OS: report.Value{Value: "Windows", Raw: "version 1", Status: "collected"},
		Components: []report.Component{{Type: "memory", Name: v("DIMM 1"), Attributes: map[string]report.Value{"capacity_bytes": v("8589934592")}}, {Type: "disk", Model: v("SSD")}},
	}}
}
func raw(t *testing.T, r report.Report) []byte {
	t.Helper()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func openTest(t *testing.T, path string) *Store {
	t.Helper()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCompareIgnoresEvidenceAndOrdering(t *testing.T) {
	a := fixture("a", time.Now())
	b := fixture("b", a.CollectedAt.Add(time.Minute))
	b.CollectorVersion = "other"
	b.Device.Manufacturer.Source = "registry"
	b.Device.Manufacturer.Observations = []report.Observation{{Source: "WMI", Status: "error", Raw: "unavailable"}}
	b.Device.Components[0], b.Device.Components[1] = b.Device.Components[1], b.Device.Components[0]
	b.Device.Components = append(b.Device.Components, report.Component{Type: "video", Name: report.Value{Status: "unavailable"}})
	if d := compare(a, b); len(d) != 0 {
		t.Fatalf("unexpected differences: %+v", d)
	}
	b.Device.Components[1].Attributes["capacity_bytes"] = report.Value{Status: "collected", Value: "17179869184"}
	if d := compare(a, b); len(d) != 1 || !strings.Contains(d[0].After, "17179869184") {
		t.Fatalf("missing difference: %+v", d)
	}
	b.Device.OS.Raw = "version 2"
	if d := compare(a, b); len(d) != 2 {
		t.Fatalf("missing OS version change: %+v", d)
	}
}

func TestImportAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "база данных.sqlite")
	s := openTest(t, path)
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	first := fixture("first", at)
	p, id, err := s.Import(raw(t, first), 0)
	if err != nil || p.Status != "new" || id == 0 {
		t.Fatalf("initial import: %+v %d %v", p, id, err)
	}
	if err := s.SaveNotes(id, "Кабинет 7"); err != nil {
		t.Fatal(err)
	}
	p, _, err = s.Import(raw(t, first), id)
	if err != nil || p.Status != "duplicate" {
		t.Fatalf("duplicate: %+v %v", p, err)
	}
	same := fixture("second", at.Add(time.Minute))
	same.Device.Manufacturer.Source = "different"
	p, _, err = s.Import(raw(t, same), id)
	if err != nil || p.Status != "unchanged" {
		t.Fatalf("unchanged: %+v %v", p, err)
	}
	changed := fixture("third", at.Add(2*time.Minute))
	changed.Device.Model.Value = "new model"
	p, _, err = s.Import(raw(t, changed), id)
	if err != nil || p.Status != "changed" {
		t.Fatalf("changed: %+v %v", p, err)
	}
	old := fixture("old", at.Add(time.Minute))
	old.Device.Model.Value = "old model"
	p, _, err = s.Import(raw(t, old), id)
	if err != nil || p.Status != "older" {
		t.Fatalf("older: %+v %v", p, err)
	}
	if _, _, err := s.Import(raw(t, fixture("bad-target", at)), 999); err == nil {
		t.Fatal("unknown asset accepted")
	}
	if _, _, err := s.Import([]byte(`{"schema_version":1}`), 0); err == nil {
		t.Fatal("invalid report accepted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openTest(t, path)
	defer s.Close()
	assets, err := s.Assets()
	if err != nil || len(assets) != 1 || assets[0].Count != 2 || assets[0].Notes != "Кабинет 7" {
		t.Fatalf("assets: %+v %v", assets, err)
	}
	h, err := s.History(id)
	if err != nil || len(h) != 2 || h[0].ScanID != "third" || !json.Valid(h[0].Raw) {
		t.Fatalf("history: %+v %v", h, err)
	}
}

func TestHTTPImportsAndSecurity(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "db.sqlite"))
	defer s.Close()
	h := Handler(s)
	rpt := fixture("api", time.Now().UTC())
	body, _ := json.Marshal(map[string]any{"asset_id": 0, "report": rpt})
	call := func(host, origin, contentType string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "http://"+host+"/api/import", bytes.NewReader(body))
		r.Header.Set("Content-Type", contentType)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := call("evil.example:8765", "", "application/json"); w.Code != http.StatusForbidden {
		t.Fatalf("host: %d", w.Code)
	}
	if w := call("localhost:8765", "http://evil.example", "application/json"); w.Code != http.StatusForbidden {
		t.Fatalf("origin: %d", w.Code)
	}
	if w := call("localhost:8765", "", "text/plain"); w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("content: %d", w.Code)
	}
	if w := call("localhost:8765", "http://localhost:8765", "application/json"); w.Code != http.StatusOK {
		t.Fatalf("import: %d %s", w.Code, w.Body.String())
	}
	if w := call("localhost:8765", "http://localhost:8765", "application/json"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "duplicate") {
		t.Fatalf("duplicate: %d %s", w.Code, w.Body.String())
	}
}

func TestHTTPEndToEnd(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "db.sqlite"))
	defer s.Close()
	h := Handler(s)
	call := func(method, path string, data any) *httptest.ResponseRecorder {
		t.Helper()
		b, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(method, "http://127.0.0.1:8765"+path, bytes.NewReader(b))
		if method != http.MethodGet {
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w
	}
	r := fixture("api-first", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	var imported struct {
		AssetID int64 `json:"asset_id"`
	}
	w := call(http.MethodPost, "/api/import", map[string]any{"asset_id": 0, "report": r})
	if err := json.Unmarshal(w.Body.Bytes(), &imported); err != nil || imported.AssetID != 1 {
		t.Fatalf("import: %s %v", w.Body, err)
	}
	call(http.MethodPut, "/api/assets/1", map[string]string{"notes": "Тест"})
	var history []struct {
		Report report.Report `json:"report"`
	}
	w = call(http.MethodGet, "/api/assets/1", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &history); err != nil || len(history) != 1 || history[0].Report.ScanID != "api-first" {
		t.Fatalf("history: %s %v", w.Body, err)
	}
	r.ScanID = "api-second"
	r.CollectedAt = r.CollectedAt.Add(time.Minute)
	r.Device.Model.Value = "another"
	var p Preview
	w = call(http.MethodPost, "/api/preview", map[string]any{"asset_id": 1, "report": r})
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || p.Status != "changed" || len(p.Differences) != 1 {
		t.Fatalf("preview: %s %v", w.Body, err)
	}
	w = call(http.MethodGet, "/api/assets", nil)
	if !strings.Contains(w.Body.String(), "Тест") {
		t.Fatalf("notes missing: %s", w.Body)
	}
}

// Reports made by the earlier collector must remain viewable/importable.
func TestExistingReports(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "db.sqlite"))
	defer s.Close()
	for _, name := range []string{"gorig-report.json", "gorig-report-test.json"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", name))
		if err != nil {
			t.Fatal(err)
		}
		p, _, err := s.Import(raw, 0)
		if err != nil || p.Status != "new" {
			t.Fatalf("%s: %+v %v", name, p, err)
		}
	}
}
