// Package summary строит payload'ы /api/summary, /api/today, /api/day —
// побайтно-семантический паритет с build_summary/_day_payload из app.py
// (группировка sub-серверов по name, наложение графиков, confirmed-down минуты).
package summary

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"xray-status/internal/config"
	"xray-status/internal/geo"
	"xray-status/internal/store"
	"xray-status/internal/sub"
)

// pollIntervalFor — действующий интервал (сек): реальный интервал проверок
// чекера (checker_interval, пишет поллер), иначе фолбэк POLL_INTERVAL.
func pollIntervalFor(st *store.Store, cfg config.Config) int {
	if v := st.GetSetting("checker_interval", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 10 {
			return n
		}
	}
	return cfg.PollInterval
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }

type member struct {
	sid     string
	online  int
	latency int
	ts      int64
	seq     int
}

type dayRec struct{ up, down, downConf int }

// BuildSummary — основной payload страницы. admin=true добавляет скрытые/
// отсутствующие серверы и поля hidden/absent.
func BuildSummary(st *store.Store, cfg config.Config, admin bool) (map[string]any, error) {
	loc, err := time.LoadLocation(cfg.TZ)
	if err != nil {
		loc = time.UTC
	}
	nowLocal := time.Now().In(loc)

	dayList := make([]string, 0, cfg.Days)
	for i := cfg.Days - 1; i >= 0; i-- {
		dayList = append(dayList, nowLocal.AddDate(0, 0, -i).Format("2006-01-02"))
	}
	pollInterval := pollIntervalFor(st, cfg)
	minPerSample := float64(pollInterval) / 60.0

	servers, err := st.CurrentRows()
	if err != nil {
		return nil, err
	}
	daily, err := st.DailyRows()
	if err != nil {
		return nil, err
	}
	hidden, err := st.HiddenSet()
	if err != nil {
		return nil, err
	}
	lastPollTS := st.LastPollTS()
	maint, err := st.MaintenanceNames(time.Now().Unix())
	if err != nil {
		return nil, err
	}
	// Окна обслуживания (name -> to_ts; 0 = без срока) — чтобы показать на странице
	// примерное время и не выводить статистику по серверу во время работ.
	maintWin := map[string]int64{}
	if ams, e := st.ActiveMaintenance(time.Now().Unix()); e == nil {
		for _, m := range ams {
			if cur, ok := maintWin[m.Name]; !ok || (m.ToTS != 0 && m.ToTS > cur) {
				maintWin[m.Name] = m.ToTS
			}
		}
	}

	bySid := map[string]map[string]dayRec{}
	for _, r := range daily {
		m := bySid[r.SID]
		if m == nil {
			m = map[string]dayRec{}
			bySid[r.SID] = m
		}
		m[r.Day] = dayRec{up: r.Up, down: r.Down, downConf: r.DownConf}
	}

	// Идентичность сервера — stableId, как в оригинальном xray-checker: одинаковая
	// ремарка в разных подписках = РАЗНЫЕ серверы, каждый своей плиткой. Имя
	// остаётся как есть и может повторяться (порядок групп — по минимальному seq).
	groups := map[string][]member{}
	nameOf := map[string]string{}
	for _, r := range servers {
		groups[r.SID] = append(groups[r.SID],
			member{sid: r.SID, online: r.Online, latency: r.Latency, ts: r.TS, seq: r.Seq})
		nameOf[r.SID] = r.Name
	}
	type ng struct {
		name    string
		members []member
		minSeq  int
	}
	ordered := make([]ng, 0, len(groups))
	for sid, ms := range groups {
		minSeq := math.MaxInt
		for _, m := range ms {
			if m.seq < minSeq {
				minSeq = m.seq
			}
		}
		ordered = append(ordered, ng{name: nameOf[sid], members: ms, minSeq: minSeq})
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].minSeq < ordered[j].minSeq })

	outServers := []any{}
	var totUp, totTotal, totDownMin, onlineCount, visibleCount, maintCount int
	var latVals []int
	var lastTS int64

	for _, g := range ordered {
		days := make([]any, 0, len(dayList))
		sUp, sTotal, sDownMin := 0, 0, 0

		for _, d := range dayList {
			sumUp, sumTotal, sumDownConf, nWithData := 0, 0, 0, 0
			for _, m := range g.members {
				if rec, ok := bySid[m.sid][d]; ok {
					nWithData++
					sumUp += rec.up
					sumTotal += rec.up + rec.down
					sumDownConf += rec.downConf
				}
			}
			var uptime any
			downMinD := 0
			if sumTotal > 0 {
				uptime = round2(float64(sumUp) / float64(sumTotal) * 100)
				downMinD = int(math.Round(float64(sumDownConf) / float64(nWithData) * minPerSample))
				sUp += sumUp
				sTotal += sumTotal
				sDownMin += downMinD
			} else {
				uptime = nil
			}
			label := dayLabel(d)
			hasData := sumTotal > 0
			days = append(days, map[string]any{
				"date": d, "label": label, "uptime": uptime,
				"downMin": downMinD, "hasData": hasData,
			})
		}

		// Канонический член — самый свежий по ts.
		ms := append([]member(nil), g.members...)
		sort.SliceStable(ms, func(i, j int) bool { return ms[i].ts > ms[j].ts })
		canon := ms[0]

		isHidden := hidden[g.name]
		isMaint := maint[g.name]
		active := lastPollTS == 0
		if !active {
			for _, m := range g.members {
				if m.ts >= lastPollTS {
					active = true
					break
				}
			}
		}
		visiblePublic := active && !isHidden
		if !admin && !visiblePublic {
			continue
		}

		var up30 any
		if sTotal > 0 {
			up30 = round2(float64(sUp) / float64(sTotal) * 100)
		} else {
			up30 = nil
		}

		if visiblePublic && isMaint {
			maintCount++
		}
		if visiblePublic && !isMaint {
			visibleCount++
			if canon.ts > lastTS {
				lastTS = canon.ts
			}
			if canon.online == 1 {
				onlineCount++
				if canon.latency > 0 {
					latVals = append(latVals, canon.latency)
				}
			}
			totUp += sUp
			totTotal += sTotal
			totDownMin += sDownMin
		}

		// Имя для показа — без тега разведения дублей (тег нужен только для
		// поштучного управления из бота и фильтрации подписки).
		base := sub.StripTag(g.name)
		cc := geo.DetectCountry(base)
		entry := map[string]any{
			"sid":         canon.sid,
			"name":        geo.DisplayName(base, cc),
			"cc":          cc,
			"online":      canon.online == 1,
			"latencyMs":   canon.latency,
			"uptime30":    up30,
			"downMin30":   sDownMin,
			"days":        days,
			"members":     len(g.members),
			"maintenance": isMaint,
		}
		if isMaint {
			entry["maintTo"] = maintWin[g.name]
		}
		if admin {
			entry["hidden"] = isHidden
			entry["absent"] = !active
		}
		outServers = append(outServers, entry)
	}

	avgLat := 0
	if len(latVals) > 0 {
		sum := 0
		for _, v := range latVals {
			sum += v
		}
		avgLat = int(math.Round(float64(sum) / float64(len(latVals))))
	}
	var totUptime any
	if totTotal > 0 {
		totUptime = round2(float64(totUp) / float64(totTotal) * 100)
	} else {
		totUptime = nil
	}
	var lastCheck any
	if lastTS > 0 {
		lastCheck = time.Unix(lastTS, 0).In(loc).Format("2006-01-02 15:04")
	} else {
		lastCheck = nil
	}

	incs, err := st.ActiveIncidents()
	if err != nil {
		return nil, err
	}
	incidents := make([]any, 0, len(incs))
	for _, in := range incs {
		// затронутые: имя без флаг-эмодзи + код страны (флаг рисуется локальным
		// SVG на странице, как в основном списке — без «DE»-букв на Windows).
		affList := make([]any, 0, len(in.Affected))
		for _, an := range in.Affected {
			ab := sub.StripTag(an)
			acc := geo.DetectCountry(ab)
			affList = append(affList, map[string]any{"name": geo.DisplayName(ab, acc), "cc": acc})
		}
		ups, _ := st.IncidentUpdates(in.ID)
		upArr := make([]any, 0, len(ups))
		for _, u := range ups {
			upArr = append(upArr, map[string]any{"ts": u.TS, "status": u.Status, "body": u.Body})
		}
		incidents = append(incidents, map[string]any{
			"id": in.ID, "title": in.Title, "severity": in.Severity,
			"status": in.Status, "affected": affList, "startedTs": in.StartedTS,
			"updates": upArr,
		})
	}

	return map[string]any{
		"title":        st.GetSetting("title", cfg.Title),
		"subtitle":     st.GetSetting("subtitle", cfg.Subtitle),
		"days":         cfg.Days,
		"pollInterval": pollInterval,
		"generatedAt":  nowLocal.Format("2006-01-02 15:04"),
		"lastCheck":    lastCheck,
		"lastCheckTs":  lastTS,
		"servers":      outServers,
		"incidents":    incidents,
		"totals": map[string]any{
			"online":      onlineCount,
			"total":       visibleCount,
			"uptime30":    totUptime,
			"avgLatency":  avgLat,
			"downMin30":   totDownMin,
			"maintenance": maintCount,
		},
	}, nil
}

// dayLabel: "22" -> "22 июн" (с сохранением ведущего нуля, как в app.py).
func dayLabel(d string) string {
	parts := strings.Split(d, "-")
	if len(parts) != 3 {
		return d
	}
	var mo int
	for _, c := range parts[1] {
		mo = mo*10 + int(c-'0')
	}
	if mo < 1 || mo > 12 {
		return d
	}
	return parts[2] + " " + geo.RUMonths[mo]
}
