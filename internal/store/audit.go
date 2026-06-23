package store

import "time"

// AddAudit пишет управляющее действие в журнал (кто/что/над чем/результат).
func (s *Store) AddAudit(actor int64, action, target, params, result string) error {
	_, err := s.exec(`INSERT INTO audit(ts,actor,action,target,params,result) VALUES(?,?,?,?,?,?)`,
		time.Now().Unix(), actor, action, target, params, result)
	return err
}

type AuditRow struct {
	TS                             int64
	Actor                          int64
	Action, Target, Params, Result string
}

// RecentAudit — последние N записей журнала (свежие сверху).
func (s *Store) RecentAudit(limit int) ([]AuditRow, error) {
	rows, err := s.query(`SELECT ts,actor,action,target,params,result FROM audit ORDER BY ts DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		var a AuditRow
		if err := rows.Scan(&a.TS, &a.Actor, &a.Action, &a.Target, &a.Params, &a.Result); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
