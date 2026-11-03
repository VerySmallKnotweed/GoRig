package base

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"GoRig/internal/report"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

type Store struct {
	db *sql.DB
	mu sync.Mutex
}

type Asset struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Notes          string `json:"notes"`
	Identification string `json:"identification"`
	DeviceType     string `json:"device_type"`
	Status         string `json:"status"`
	LatestAt       string `json:"latest_at"`
	Count          int    `json:"count"`
	OrganizationID int64  `json:"organization_id"`
	CPUVendor      string `json:"cpu_vendor"`
	CPU            string `json:"cpu"`
}
type Inspection struct {
	ID          int64           `json:"id"`
	ScanID      string          `json:"scan_id"`
	CollectedAt string          `json:"collected_at"`
	ImportedAt  string          `json:"imported_at"`
	Raw         json.RawMessage `json:"report"`
}

type Preview struct {
	Status           string       `json:"status"`
	Differences      []Difference `json:"differences"`
	LatestAt         string       `json:"latest_at,omitempty"`
	DuplicateAssetID int64        `json:"duplicate_asset_id,omitempty"`
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA foreign_keys=ON;
CREATE TABLE IF NOT EXISTS organizations (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL COLLATE NOCASE UNIQUE);
CREATE TABLE IF NOT EXISTS assets (id INTEGER PRIMARY KEY, name TEXT NOT NULL, notes TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'in_use');
CREATE TABLE IF NOT EXISTS inspections (id INTEGER PRIMARY KEY, asset_id INTEGER NOT NULL REFERENCES assets(id), scan_id TEXT NOT NULL UNIQUE, collected_at TEXT NOT NULL, imported_at TEXT NOT NULL, report_json TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS inspections_asset ON inspections(asset_id, id);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	// Existing databases predate the status column. The migration is additive;
	// all existing computers remain in use, with history and notes untouched.
	rows, err := db.Query("PRAGMA table_info(assets)")
	if err != nil {
		db.Close()
		return nil, err
	}
	hasStatus, hasOrganization, hasIdentification, hasDeviceType := false, false, false, false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, typ string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			db.Close()
			return nil, err
		}
		if name == "status" {
			hasStatus = true
		}
		if name == "organization_id" {
			hasOrganization = true
		}
		if name == "identification" {
			hasIdentification = true
		}
		if name == "device_type" {
			hasDeviceType = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		db.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		db.Close()
		return nil, err
	}
	if !hasStatus {
		if _, err := db.Exec("ALTER TABLE assets ADD COLUMN status TEXT NOT NULL DEFAULT 'in_use'"); err != nil {
			db.Close()
			return nil, err
		}
	}
	if !hasOrganization {
		if _, err := db.Exec("ALTER TABLE assets ADD COLUMN organization_id INTEGER REFERENCES organizations(id) ON DELETE SET NULL"); err != nil {
			db.Close()
			return nil, err
		}
	}
	if !hasIdentification {
		if _, err := db.Exec("ALTER TABLE assets ADD COLUMN identification TEXT NOT NULL DEFAULT ''"); err != nil {
			db.Close()
			return nil, err
		}
	}
	if !hasDeviceType {
		if _, err := db.Exec("ALTER TABLE assets ADD COLUMN device_type TEXT NOT NULL DEFAULT ''"); err != nil {
			db.Close()
			return nil, err
		}
	}
	// Never reuse a deleted asset ID: a stale browser tab must not accidentally
	// edit or delete a different card created afterwards.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS asset_sequence (id INTEGER PRIMARY KEY CHECK (id=1), last_id INTEGER NOT NULL);
INSERT INTO asset_sequence(id,last_id) SELECT 1,COALESCE(MAX(id),0) FROM assets WHERE true
ON CONFLICT(id) DO UPDATE SET last_id=MAX(last_id,excluded.last_id);`); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Assets() ([]Asset, error) {
	rows, err := s.db.Query(`SELECT a.id, a.name, a.notes, a.identification, a.device_type, a.status, COALESCE((SELECT collected_at FROM inspections WHERE asset_id=a.id ORDER BY id DESC LIMIT 1),''), (SELECT COUNT(*) FROM inspections WHERE asset_id=a.id), COALESCE(a.organization_id,0), COALESCE((SELECT report_json FROM inspections WHERE asset_id=a.id ORDER BY id DESC LIMIT 1),'') FROM assets a ORDER BY a.name, a.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Asset{}
	for rows.Next() {
		var a Asset
		var raw string
		if err := rows.Scan(&a.ID, &a.Name, &a.Notes, &a.Identification, &a.DeviceType, &a.Status, &a.LatestAt, &a.Count, &a.OrganizationID, &raw); err != nil {
			return nil, err
		}
		if raw != "" {
			if r, err := decodeReport([]byte(raw)); err == nil {
				for _, c := range r.Device.Components {
					if c.Type != "processor" {
						continue
					}
					a.CPU = value(c.Name)
					if a.CPU == "" {
						a.CPU = value(c.Model)
					}
					vendor := strings.ToLower(value(c.Manufacturer) + " " + a.CPU)
					if strings.Contains(vendor, "intel") {
						a.CPUVendor = "intel"
					} else if strings.Contains(vendor, "amd") || strings.Contains(vendor, "ryzen") {
						a.CPUVendor = "amd"
					}
					break
				}
			}
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) History(assetID int64) ([]Inspection, error) {
	var exists int
	if err := s.db.QueryRow("SELECT 1 FROM assets WHERE id=?", assetID).Scan(&exists); err != nil {
		return nil, err
	}
	rows, err := s.db.Query("SELECT id,scan_id,collected_at,imported_at,report_json FROM inspections WHERE asset_id=? ORDER BY id DESC", assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Inspection{}
	for rows.Next() {
		var x Inspection
		var raw string
		if err := rows.Scan(&x.ID, &x.ScanID, &x.CollectedAt, &x.ImportedAt, &raw); err != nil {
			return nil, err
		}
		x.Raw = json.RawMessage(raw)
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Store) SaveNotes(id int64, notes string) error {
	return s.SaveDetails(id, notes, nil)
}

type AssetDetails struct {
	Notes          string  `json:"notes"`
	Status         *string `json:"status"`
	Identification *string `json:"identification"`
	DeviceType     *string `json:"device_type"`
	// Nil preserves the existing association; zero explicitly clears it.
	OrganizationID *int64 `json:"organization_id"`
}

func validateDetails(d AssetDetails) error {
	if len(d.Notes) > 100000 {
		return errors.New("заметка слишком большая")
	}
	if d.Identification != nil && utf8.RuneCountInString(*d.Identification) > 80 {
		return errors.New("идентификация не должна превышать 80 символов")
	}
	if d.DeviceType != nil && *d.DeviceType != "" && *d.DeviceType != "laptop" && *d.DeviceType != "desktop" && *d.DeviceType != "all_in_one" && *d.DeviceType != "other" {
		return errors.New("неверный тип устройства")
	}
	if d.Status != nil && *d.Status != "in_use" && *d.Status != "reserve" && *d.Status != "retired" {
		return errors.New("неверный статус компьютера")
	}
	if d.OrganizationID != nil && *d.OrganizationID < 0 {
		return errors.New("неверная организация")
	}
	return nil
}

type executor interface {
	Exec(string, ...any) (sql.Result, error)
}

func saveDetails(q executor, id int64, d AssetDetails) error {
	if err := validateDetails(d); err != nil {
		return err
	}
	var status, org, identification, deviceType any
	if d.Status != nil {
		status = *d.Status
	}
	if d.OrganizationID != nil {
		org = *d.OrganizationID
	}
	if d.Identification != nil {
		identification = strings.TrimSpace(*d.Identification)
	}
	if d.DeviceType != nil {
		deviceType = *d.DeviceType
	}
	res, err := q.Exec(`UPDATE assets SET notes=?, status=COALESCE(?,status), organization_id=CASE WHEN ? IS NULL THEN organization_id WHEN ?=0 THEN NULL ELSE ? END, identification=COALESCE(?,identification), device_type=COALESCE(?,device_type) WHERE id=?`, d.Notes, status, org, org, org, identification, deviceType, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
func (s *Store) SaveDetails(id int64, notes string, status *string) error {
	return s.SaveAssetDetails(id, AssetDetails{Notes: notes, Status: status})
}
func (s *Store) SaveAssetDetails(id int64, d AssetDetails) error { return saveDetails(s.db, id, d) }

type Organization struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (s *Store) Organizations() ([]Organization, error) {
	rows, err := s.db.Query("SELECT id,name FROM organizations ORDER BY name,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Organization{}
	for rows.Next() {
		var o Organization
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
func validOrganizationName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 300 {
		return "", errors.New("укажите название организации (не более 300 байт)")
	}
	return name, nil
}
func (s *Store) CreateOrganization(name string) (int64, error) {
	name, err := validOrganizationName(name)
	if err != nil {
		return 0, err
	}
	res, err := s.db.Exec("INSERT INTO organizations(name) VALUES(?)", name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
func (s *Store) RenameOrganization(id int64, name string) error {
	name, err := validOrganizationName(name)
	if err != nil {
		return err
	}
	res, err := s.db.Exec("UPDATE organizations SET name=? WHERE id=?", name, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
func (s *Store) DeleteOrganization(id int64) error {
	res, err := s.db.Exec("DELETE FROM organizations WHERE id=?", id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Delete removes both the card and its history atomically, including scan IDs.
func (s *Store) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM inspections WHERE asset_id=?", id); err != nil {
		return err
	}
	res, err := tx.Exec("DELETE FROM assets WHERE id=?", id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

type querier interface{ QueryRow(string, ...any) *sql.Row }

func preview(q querier, r report.Report, assetID int64) (Preview, error) {
	p := Preview{Differences: []Difference{}}
	var existingID int64
	err := q.QueryRow("SELECT asset_id FROM inspections WHERE scan_id=?", r.ScanID).Scan(&existingID)
	if err == nil {
		p.Status = "duplicate"
		p.DuplicateAssetID = existingID
		return p, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return p, err
	}
	if assetID == 0 {
		p.Status = "new"
		return p, nil
	}
	var raw string
	err = q.QueryRow("SELECT report_json FROM inspections WHERE asset_id=? ORDER BY id DESC LIMIT 1", assetID).Scan(&raw)
	if err != nil {
		return p, err
	}
	old, err := decodeReport([]byte(raw))
	if err != nil {
		return p, fmt.Errorf("сохранённый отчёт повреждён: %w", err)
	}
	p.LatestAt = old.CollectedAt.Format(time.RFC3339Nano)
	p.Differences = compare(old, r)
	if r.CollectedAt.Before(old.CollectedAt) {
		p.Status = "older"
	} else if len(p.Differences) == 0 {
		p.Status = "unchanged"
	} else {
		p.Status = "changed"
	}
	return p, nil
}
func (s *Store) Preview(raw []byte, assetID int64) (Preview, error) {
	r, err := decodeReport(raw)
	if err != nil {
		return Preview{}, err
	}
	return preview(s.db, r, assetID)
}
func (s *Store) Import(raw []byte, assetID int64) (Preview, int64, error) {
	return s.ImportWithDetails(raw, assetID, nil)
}
func (s *Store) ImportWithDetails(raw []byte, assetID int64, details *AssetDetails) (Preview, int64, error) {
	if details != nil {
		if err := validateDetails(*details); err != nil {
			return Preview{}, 0, err
		}
	}
	r, err := decodeReport(raw)
	if err != nil {
		return Preview{}, 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return Preview{}, 0, err
	}
	defer tx.Rollback()
	p, err := preview(tx, r, assetID)
	if err != nil {
		return p, 0, err
	}
	if p.Status != "new" && p.Status != "changed" {
		return p, assetID, nil
	}
	if assetID == 0 {
		name := value(r.Device.ComputerName)
		if name == "" {
			name = "Без имени"
		}
		if _, err := tx.Exec("UPDATE asset_sequence SET last_id=MAX(last_id, (SELECT COALESCE(MAX(id),0) FROM assets))+1 WHERE id=1"); err != nil {
			return p, 0, err
		}
		if err := tx.QueryRow("SELECT last_id FROM asset_sequence WHERE id=1").Scan(&assetID); err != nil {
			return p, 0, err
		}
		if _, err := tx.Exec("INSERT INTO assets(id,name) VALUES(?,?)", assetID, name); err != nil {
			return p, 0, err
		}
	} else {
		// The asset identity is explicitly chosen by the user; update its display name.
		if name := value(r.Device.ComputerName); name != "" {
			if _, err := tx.Exec("UPDATE assets SET name=? WHERE id=?", name, assetID); err != nil {
				return p, 0, err
			}
		}
	}
	if details != nil {
		if err := saveDetails(tx, assetID, *details); err != nil {
			return p, 0, err
		}
	}
	_, err = tx.Exec("INSERT INTO inspections(asset_id,scan_id,collected_at,imported_at,report_json) VALUES(?,?,?,?,?)", assetID, r.ScanID, r.CollectedAt.UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), string(raw))
	if err != nil {
		return p, 0, err
	}
	if err := tx.Commit(); err != nil {
		return p, 0, err
	}
	return p, assetID, nil
}
