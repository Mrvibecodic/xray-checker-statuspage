package summary

import (
	"math"
	"sort"
	"time"

	"xray-status/internal/config"
	"xray-status/internal/geo"
	"xray-status/internal/store"
)

// BuildToday — детализация за сегодня (с полуночи в TZ до текущего момента).
func BuildToday(st *store.Store, cfg config.Config, sid string) (map[string]any, error) {
	loc := locOf(cfg)
	ds := midnight(time.Now().In(loc))
	return dayPayload(st, cfg, loc, sid, ds, true)
}

// BuildDay — детализация за указанную дату YYYY-MM-DD. Невалидная дата → сегодня.
func BuildDay(st *store.Store, cfg config.Config, sid, dateStr string) (map[string]any, error) {
	loc := locOf(cfg)
	d, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return BuildToday(st, cfg, sid)
	}
	ds := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc).Unix()
	today0 := midnight(time.Now().In(loc))
	return dayPayload(st, cfg, loc, sid, ds, ds == today0)
}

func locOf(cfg config.Config) *time.Location {
	loc, err := time.LoadLocation(cfg.TZ)
	if err != nil {
		return time.UTC
	}
	return loc
}

func midnight(t time.Time) int64 {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Unix()
}

type bucket struct {
	online  int
	latency int
}

func dayPayload(st *store.Store, cfg config.Config, loc *time.Location, sid string, ds int64, isToday bool) (map[string]any, error) {
	end := ds + 86400
	upper := end
	if isToday {
		upper = time.Now().Unix()
	}

	members, err := st.GroupMembers(sid)
	if err != nil {
		return nil, err
	}
	rows, err := st.SamplesForGroup(members, ds, end)
	if err != nil {
		return nil, err
	}

	// Роллап по ts: «любой online → online», латентность = минимум среди online.
	buckets := map[int64]*bucket{}
	for _, r := range rows {
		b := buckets[r.TS]
		if b == nil {
			lat := 0
			if r.Online == 1 {
				lat = r.Latency
			}
			buckets[r.TS] = &bucket{online: r.Online, latency: lat}
			continue
		}
		if r.Online == 1 {
			if b.online == 0 {
				b.online = 1
				if r.Latency > 0 {
					b.latency = r.Latency
				} else {
					b.latency = 0
				}
			} else if r.Latency > 0 && (b.latency == 0 || r.Latency < b.latency) {
				b.latency = r.Latency
			}
		}
	}

	tss := make([]int64, 0, len(buckets))
	for ts := range buckets {
		tss = append(tss, ts)
	}
	sort.Slice(tss, func(i, j int) bool { return tss[i] < tss[j] })

	samples := make([]any, 0, len(tss))
	var pings []int
	errors := 0
	for _, ts := range tss {
		b := buckets[ts]
		online := b.online == 1
		samples = append(samples, map[string]any{
			"ts": ts, "online": online, "latency": b.latency,
		})
		if !online {
			errors++
		}
		if online && b.latency > 0 {
			pings = append(pings, b.latency)
		}
	}

	pmin, pavg, pmax := 0, 0, 0
	if len(pings) > 0 {
		pmin, pmax = pings[0], pings[0]
		sum := 0
		for _, v := range pings {
			if v < pmin {
				pmin = v
			}
			if v > pmax {
				pmax = v
			}
			sum += v
		}
		pavg = int(math.Round(float64(sum) / float64(len(pings))))
	}

	dt := time.Unix(ds, 0).In(loc)
	label := "Сегодня"
	if !isToday {
		label = pad2(dt.Day()) + " " + geo.RUMonths[int(dt.Month())]
	}

	return map[string]any{
		"dayStart":     ds,
		"now":          upper,
		"isToday":      isToday,
		"dayLabel":     label,
		"pollInterval": pollIntervalFor(st, cfg),
		"samples":      samples,
		"stats": map[string]any{
			"checks": len(samples),
			"errors": errors,
			"pmin":   pmin,
			"pavg":   pavg,
			"pmax":   pmax,
		},
	}, nil
}

func pad2(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
