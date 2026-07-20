package store

import (
	"database/sql"
	"encoding/json"
	"time"
)

// --- Инциденты ---

// Допустимые статусы инцидента (поток Statuspage/Cachet).
var IncidentStatuses = []string{"investigating", "identified", "monitoring", "resolved"}

// Допустимые уровни важности.
var IncidentSeverities = []string{"minor", "major", "critical"}

type Incident struct {
	ID         int64
	Title      string
	Severity   string
	Status     string
	Affected   []string
	StartedTS  int64
	ResolvedTS int64
	Auto       bool
	CreatedBy  int64
}

type IncidentUpdate struct {
	ID     int64
	TS     int64
	Status string
	Body   string
	Author int64
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

const incidentCols = `id,title,severity,status,affected,started_ts,resolved_ts,auto,created_by`

// CreateIncident заводит инцидент (status=investigating) + первое обновление.
func (s *Store) CreateIncident(title, severity string, affected []string, body string, author int64, auto bool) (int64, error) {
	now := time.Now().Unix()
	aff, _ := json.Marshal(affected)
	id, err := s.insertID(
		`INSERT INTO incidents(title,severity,status,affected,started_ts,resolved_ts,auto,created_by)
		 VALUES(?,?,?,?,?,0,?,?)`,
		title, severity, "investigating", string(aff), now, boolToInt(auto), author)
	if err != nil {
		return 0, err
	}
	_, err = s.exec(
		`INSERT INTO incident_updates(incident_id,ts,status,body,author) VALUES(?,?,?,?,?)`,
		id, now, "investigating", body, author)
	return id, err
}

// AddIncidentUpdate меняет статус инцидента и добавляет запись в ленту.
// status=resolved проставляет resolved_ts.
func (s *Store) AddIncidentUpdate(id int64, status, body string, author int64) error {
	now := time.Now().Unix()
	if status == "resolved" {
		if _, err := s.exec(`UPDATE incidents SET status=?, resolved_ts=? WHERE id=?`, status, now, id); err != nil {
			return err
		}
	} else {
		if _, err := s.exec(`UPDATE incidents SET status=? WHERE id=?`, status, id); err != nil {
			return err
		}
	}
	_, err := s.exec(
		`INSERT INTO incident_updates(incident_id,ts,status,body,author) VALUES(?,?,?,?,?)`,
		id, now, status, body, author)
	return err
}

func scanIncidents(rows *sql.Rows, qerr error) ([]Incident, error) {
	if qerr != nil {
		return nil, qerr
	}
	defer rows.Close()
	var out []Incident
	for rows.Next() {
		var in Incident
		var aff string
		var auto int
		if err := rows.Scan(&in.ID, &in.Title, &in.Severity, &in.Status, &aff,
			&in.StartedTS, &in.ResolvedTS, &auto, &in.CreatedBy); err != nil {
			return nil, err
		}
		in.Auto = auto == 1
		_ = json.Unmarshal([]byte(aff), &in.Affected)
		out = append(out, in)
	}
	return out, rows.Err()
}

// ActiveIncidents — незакрытые, свежие сверху.
func (s *Store) ActiveIncidents() ([]Incident, error) {
	return scanIncidents(s.query(`SELECT ` + incidentCols + ` FROM incidents WHERE status!='resolved' ORDER BY started_ts DESC`))
}

// RecentIncidents — последние N инцидентов (включая закрытые).
func (s *Store) RecentIncidents(limit int) ([]Incident, error) {
	return scanIncidents(s.query(`SELECT `+incidentCols+` FROM incidents ORDER BY started_ts DESC LIMIT ?`, limit))
}

// GetIncident возвращает один инцидент по id.
func (s *Store) GetIncident(id int64) (*Incident, error) {
	var in Incident
	var aff string
	var auto int
	err := s.queryRow(`SELECT `+incidentCols+` FROM incidents WHERE id=?`, id).
		Scan(&in.ID, &in.Title, &in.Severity, &in.Status, &aff,
			&in.StartedTS, &in.ResolvedTS, &auto, &in.CreatedBy)
	if err != nil {
		return nil, err
	}
	in.Auto = auto == 1
	_ = json.Unmarshal([]byte(aff), &in.Affected)
	return &in, nil
}

// IncidentUpdates — лента обновлений инцидента (старые сверху).
func (s *Store) IncidentUpdates(id int64) ([]IncidentUpdate, error) {
	rows, err := s.query(
		`SELECT id,ts,status,body,author FROM incident_updates WHERE incident_id=? ORDER BY ts`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IncidentUpdate
	for rows.Next() {
		var u IncidentUpdate
		if err := rows.Scan(&u.ID, &u.TS, &u.Status, &u.Body, &u.Author); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// --- Обслуживание (maintenance) ---

type Maintenance struct {
	ID        int64
	Name      string
	FromTS    int64
	ToTS      int64 // 0 = бессрочно (пока вручную не закроют)
	Reason    string
	CreatedBy int64
	CreatedTS int64
}

// AddMaintenance планирует окно работ для группы серверов.
func (s *Store) AddMaintenance(name string, fromTS, toTS int64, reason string, by int64) (int64, error) {
	id, err := s.insertID(
		`INSERT INTO maintenance(name,from_ts,to_ts,reason,created_by,created_ts) VALUES(?,?,?,?,?,?)`,
		name, fromTS, toTS, reason, by, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return id, nil
}

// EndMaintenance закрывает окно сейчас.
func (s *Store) EndMaintenance(id int64) error {
	_, err := s.exec(`UPDATE maintenance SET to_ts=? WHERE id=?`, time.Now().Unix(), id)
	return err
}

// ActiveMaintenance — окна, активные на момент now (from<=now<to, либо to=0).
func (s *Store) ActiveMaintenance(now int64) ([]Maintenance, error) {
	rows, err := s.query(
		`SELECT id,name,from_ts,to_ts,reason,created_by,created_ts FROM maintenance
		 WHERE from_ts<=? AND (to_ts=0 OR to_ts>?) ORDER BY from_ts`, now, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Maintenance
	for rows.Next() {
		var m Maintenance
		if err := rows.Scan(&m.ID, &m.Name, &m.FromTS, &m.ToTS, &m.Reason, &m.CreatedBy, &m.CreatedTS); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MaintenanceNames — множество групп в обслуживании на момент now.
func (s *Store) MaintenanceNames(now int64) (map[string]bool, error) {
	ms, err := s.ActiveMaintenance(now)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, m := range ms {
		out[m.Name] = true
	}
	return out, nil
}
