package base

import (
	"database/sql"
	"encoding/json"
)

// PrintableReport is a consistent snapshot of the latest saved inspections.
// Older inspections and their raw source evidence are not needed for the
// summary document, although the latest report is used to extract hardware.
type PrintableReport struct {
	Organizations []Organization   `json:"organizations"`
	Assets        []PrintableAsset `json:"assets"`
}

type PrintableAsset struct {
	ID             int64           `json:"id"`
	Name           string          `json:"name"`
	Notes          string          `json:"notes"`
	Status         string          `json:"status"`
	OrganizationID int64           `json:"organization_id"`
	Report         json.RawMessage `json:"report"`
}

// PrintableSnapshot returns all organizations (scope -1), the unassigned
// computers (0), or one named organization (>0), all read in one transaction.
func (s *Store) PrintableSnapshot(scope int64) (PrintableReport, error) {
	out := PrintableReport{Organizations: []Organization{}, Assets: []PrintableAsset{}}
	if scope < -1 {
		return out, sql.ErrNoRows
	}
	tx, err := s.db.Begin()
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	rows, err := tx.Query("SELECT id,name FROM organizations WHERE ?=-1 OR id=? ORDER BY name,id", scope, scope)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var o Organization
		if err = rows.Scan(&o.ID, &o.Name); err != nil {
			rows.Close()
			return out, err
		}
		out.Organizations = append(out.Organizations, o)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	if err = rows.Close(); err != nil {
		return out, err
	}
	if scope > 0 && len(out.Organizations) == 0 {
		return out, sql.ErrNoRows
	}
	if scope <= 0 {
		out.Organizations = append(out.Organizations, Organization{ID: 0, Name: "Непривязанные"})
	}
	rows, err = tx.Query(`SELECT a.id,a.name,a.notes,a.status,COALESCE(a.organization_id,0),
		COALESCE((SELECT report_json FROM inspections WHERE asset_id=a.id ORDER BY id DESC LIMIT 1),'null')
		FROM assets a WHERE ?=-1 OR COALESCE(a.organization_id,0)=? ORDER BY a.name,a.id`, scope, scope)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var a PrintableAsset
		var raw string
		if err = rows.Scan(&a.ID, &a.Name, &a.Notes, &a.Status, &a.OrganizationID, &raw); err != nil {
			rows.Close()
			return out, err
		}
		a.Report = json.RawMessage(raw)
		out.Assets = append(out.Assets, a)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	if err = rows.Close(); err != nil {
		return out, err
	}
	return out, tx.Commit()
}
