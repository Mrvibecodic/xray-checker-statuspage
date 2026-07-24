package store

import (
	"strconv"

	"xray-status/internal/checker"
)

// PollWrite — транзакционная запись одного цикла опроса (паритет poll_once из
// app.py): обновляет current, аккумулирует daily (с подтверждённым down_conf),
// пишет samples, затем автоочистка «призраков» и ретеншн.
//
// Решение «писать или пропустить» (отсев глобального сбоя) принимается выше, в
// поллере; сюда попадают только циклы, которые надо записать.
type PollWriteParams struct {
	Now              int64
	Today            string // YYYY-MM-DD в нужной TZ
	PollInterval     int
	Autoclean        bool
	StaleAfterHours  int
	CutoffDay        string // YYYY-MM-DD: daily старше — удалить
	SampleRetainDays int
}

func (s *Store) PollWrite(proxies []checker.Proxy, p PollWriteParams) (cleaned []string, err error) {
	tx, err := s.begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for seq, px := range proxies {
		sid := px.StableID
		if sid == "" {
			continue
		}
		name := px.Name
		if name == "" {
			name = sid
		}
		online := 0
		if px.Online {
			online = 1
		}
		latency := px.LatencyMs
		if latency < 0 {
			latency = 0
		}

		// down_conf: подтверждённый простой — два офлайна подряд в пределах 2 интервалов.
		var prevOnline int
		var prevTS int64
		hasPrev := false
		row := tx.QueryRow(`SELECT online, ts FROM current WHERE sid=?`, sid)
		if e := row.Scan(&prevOnline, &prevTS); e == nil {
			hasPrev = true
		}
		downConf := 0
		if online == 0 && hasPrev && prevOnline == 0 && prevTS > 0 &&
			(p.Now-prevTS) <= int64(p.PollInterval*2) {
			downConf = 1
		}

		if _, err = tx.Exec(`INSERT INTO current(sid,name,grp,online,latency,ts,seq)
			VALUES(?,?,?,?,?,?,?)
			ON CONFLICT(sid) DO UPDATE SET
			  name=excluded.name, grp=excluded.grp, online=excluded.online,
			  latency=excluded.latency, ts=excluded.ts, seq=excluded.seq`,
			sid, name, px.GroupName, online, latency, p.Now, seq); err != nil {
			return nil, err
		}

		up := online
		down := 1 - online
		latSum, latCnt := 0, 0
		if online == 1 && latency > 0 {
			latSum, latCnt = latency, 1
		}
		if _, err = tx.Exec(`INSERT INTO daily(day,sid,up,down,lat_sum,lat_cnt,down_conf)
			VALUES(?,?,?,?,?,?,?)
			ON CONFLICT(day,sid) DO UPDATE SET
			  up=up+excluded.up, down=down+excluded.down,
			  lat_sum=lat_sum+excluded.lat_sum, lat_cnt=lat_cnt+excluded.lat_cnt,
			  down_conf=down_conf+excluded.down_conf`,
			p.Today, sid, up, down, latSum, latCnt, downConf); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(`INSERT INTO samples(ts,sid,online,latency) VALUES(?,?,?,?)`,
			p.Now, sid, online, latency); err != nil {
			return nil, err
		}
	}

	// Автоочистка «призраков»: группы (по name), у которых ВСЕ члены старше порога.
	if len(proxies) > 0 && p.StaleAfterHours > 0 && p.Autoclean {
		staleCut := p.Now - int64(p.StaleAfterHours)*3600
		type m struct {
			sid string
			ts  int64
		}
		byName := map[string][]m{}
		rows, e := tx.Query(`SELECT name, sid, ts FROM current`)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var nm, sid string
			var ts int64
			if e := rows.Scan(&nm, &sid, &ts); e != nil {
				rows.Close()
				return nil, e
			}
			byName[nm] = append(byName[nm], m{sid, ts})
		}
		rows.Close()
		for _, items := range byName {
			allStale := true
			for _, it := range items {
				if it.ts >= staleCut {
					allStale = false
					break
				}
			}
			if allStale {
				for _, it := range items {
					cleaned = append(cleaned, it.sid)
				}
			}
		}
		for _, sid := range cleaned {
			if _, err = tx.Exec(`DELETE FROM current WHERE sid=?`, sid); err != nil {
				return nil, err
			}
			if _, err = tx.Exec(`DELETE FROM daily WHERE sid=?`, sid); err != nil {
				return nil, err
			}
			if _, err = tx.Exec(`DELETE FROM samples WHERE sid=?`, sid); err != nil {
				return nil, err
			}
		}
	}

	// Ретеншн.
	if _, err = tx.Exec(`DELETE FROM daily WHERE day < ?`, p.CutoffDay); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`DELETE FROM samples WHERE ts < ?`,
		p.Now-int64(p.SampleRetainDays)*86400); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO settings(k,v) VALUES('last_poll_ts',?)
		ON CONFLICT(k) DO UPDATE SET v=excluded.v`, strconv.FormatInt(p.Now, 10)); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return cleaned, nil
}

// DeleteServer удаляет всю группу: одноимённые sid, а для балансировочной
// группы — ещё и все sid с тем же grp. Узлы балансира носят РАЗНЫЕ имена
// («… | proxy», «… | proxy-2»), их объединяет только grp; без этого кнопка
// удаления сносила один узел, и группа «воскресала» в списке отсутствующих.
// Возврат — число удалённых sid.
func (s *Store) DeleteServer(sid string) (int, error) {
	if sid == "" {
		return 0, nil
	}
	tx, err := s.begin()
	if err != nil {
		return 0, err
	}
	var name, grp string
	if err := tx.QueryRow(`SELECT name, COALESCE(grp,'') FROM current WHERE sid=?`, sid).Scan(&name, &grp); err != nil {
		_ = tx.Rollback()
		return 0, nil
	}
	q, args := `SELECT sid FROM current WHERE name=?`, []any{name}
	if grp != "" {
		q, args = `SELECT sid FROM current WHERE name=? OR grp=?`, []any{name, grp}
	}
	rows, err := tx.Query(q, args...)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	var members []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		members = append(members, s)
	}
	rows.Close()
	for _, s := range members {
		_, _ = tx.Exec(`DELETE FROM current WHERE sid=?`, s)
		_, _ = tx.Exec(`DELETE FROM daily WHERE sid=?`, s)
		_, _ = tx.Exec(`DELETE FROM samples WHERE sid=?`, s)
	}
	_, _ = tx.Exec(`DELETE FROM hidden WHERE name=?`, name)
	if grp != "" {
		// скрытие групп ключуется по имени группы — подчистим и его
		_, _ = tx.Exec(`DELETE FROM hidden WHERE name=?`, grp)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(members), nil
}

// SetHidden скрывает/показывает группу по sid.
func (s *Store) SetHidden(sid string, hide bool) (bool, error) {
	if sid == "" {
		return false, nil
	}
	var name string
	if err := s.queryRow(`SELECT name FROM current WHERE sid=?`, sid).Scan(&name); err != nil {
		return false, nil
	}
	if hide {
		q := `INSERT OR IGNORE INTO hidden(name) VALUES(?)`
		if s.d.isPG() {
			q = `INSERT INTO hidden(name) VALUES(?) ON CONFLICT DO NOTHING`
		}
		_, err := s.exec(q, name)
		return err == nil, err
	}
	_, err := s.exec(`DELETE FROM hidden WHERE name=?`, name)
	return err == nil, err
}

// SetHiddenName скрывает/показывает группу по имени (для бота: /server off|on).
func (s *Store) SetHiddenName(name string, hide bool) error {
	if hide {
		q := `INSERT OR IGNORE INTO hidden(name) VALUES(?)`
		if s.d.isPG() {
			q = `INSERT INTO hidden(name) VALUES(?) ON CONFLICT DO NOTHING`
		}
		_, err := s.exec(q, name)
		return err
	}
	_, err := s.exec(`DELETE FROM hidden WHERE name=?`, name)
	return err
}
