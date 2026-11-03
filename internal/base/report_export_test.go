package base

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrintableSnapshotAndEndpoint(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "print.sqlite"))
	defer s.Close()
	org, err := s.CreateOrganization("Школа")
	if err != nil {
		t.Fatal(err)
	}
	empty, err := s.CreateOrganization("Пустая")
	if err != nil {
		t.Fatal(err)
	}
	status := "reserve"
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_, id, err := s.ImportWithDetails(raw(t, fixture("old", baseTime)), 0, &AssetDetails{Notes: "Кабинет 5", Status: &status, OrganizationID: &org})
	if err != nil {
		t.Fatal(err)
	}
	latest := fixture("latest", baseTime.Add(time.Hour))
	latest.Device.Model.Value = "Новая модель"
	_, _, err = s.ImportWithDetails(raw(t, latest), id, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, unassigned, err := s.ImportWithDetails(raw(t, fixture("unassigned", baseTime)), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	all, err := s.PrintableSnapshot(-1)
	if err != nil || len(all.Organizations) != 3 || len(all.Assets) != 2 {
		t.Fatalf("all: %+v %v", all, err)
	}
	one, err := s.PrintableSnapshot(org)
	if err != nil || len(one.Organizations) != 1 || len(one.Assets) != 1 {
		t.Fatalf("one: %+v %v", one, err)
	}
	if one.Assets[0].ID != id || one.Assets[0].Notes != "Кабинет 5" || one.Assets[0].Status != status || !strings.Contains(string(one.Assets[0].Report), "Новая модель") || strings.Contains(string(one.Assets[0].Report), `"scan_id":"old"`) {
		t.Fatalf("latest: %+v", one.Assets[0])
	}
	zero, err := s.PrintableSnapshot(0)
	if err != nil || len(zero.Organizations) != 1 || zero.Organizations[0].Name != "Непривязанные" || len(zero.Assets) != 1 || zero.Assets[0].ID != unassigned {
		t.Fatalf("unassigned: %+v %v", zero, err)
	}
	vacant, err := s.PrintableSnapshot(empty)
	if err != nil || len(vacant.Assets) != 0 {
		t.Fatalf("empty: %+v %v", vacant, err)
	}
	if _, err = s.PrintableSnapshot(987); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing org: %v", err)
	}
	h := Handler(s)
	for _, tc := range []struct {
		url  string
		code int
	}{
		{"/api/print-report", 200}, {"/api/print-report?organization_id=0", 200}, {"/api/print-report?organization_id=-1", 400}, {"/api/print-report?organization_id=bad", 400}, {"/api/print-report?organization_id=987", 404},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://localhost:8765"+tc.url, nil))
		if w.Code != tc.code {
			t.Fatalf("%s: %d, want %d", tc.url, w.Code, tc.code)
		}
		if tc.code == 200 {
			var result PrintableReport
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
		}
	}
}
