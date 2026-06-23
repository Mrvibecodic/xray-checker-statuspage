// Package poller периодически опрашивает xray-checker и пишет историю в store.
// Логика — паритет poll_once/poller из app.py, включая confirm-then-write для
// глобального сбоя чекера (ПЛАН §10).
package poller

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"xray-status/internal/checker"
	"xray-status/internal/config"
	"xray-status/internal/store"
)

type Poller struct {
	cfg      config.Config
	client   *checker.Client
	st       *store.Store
	loc      *time.Location
	streak   int             // _alldown_streak: счётчик подтверждения глобального сбоя
	pingHigh map[string]bool // имя группы -> пинг сейчас выше порога
	onEvent  func(Event)
	mu       sync.Mutex // сериализует фоновый цикл и RunOnce из бота
}

func New(cfg config.Config, st *store.Store, client *checker.Client) *Poller {
	loc, err := time.LoadLocation(cfg.TZ)
	if err != nil {
		loc = time.UTC
	}
	return &Poller{cfg: cfg, client: client, st: st, loc: loc, pingHigh: map[string]bool{}}
}

// OnEvent регистрирует обработчик событий (смена статуса, глобальный сбой) —
// используется ботом для алертов (M2).
func (p *Poller) OnEvent(fn func(Event)) { p.onEvent = fn }

func (p *Poller) emit(e Event) {
	if p.onEvent != nil {
		p.onEvent(e)
	}
}

func (p *Poller) skipGlobalDefault() string {
	if p.cfg.GlobalOutageRatio <= 1.0 {
		return "1"
	}
	return "0"
}

func (p *Poller) interval() int {
	// Интервал берём у самого чекера (его реальная частота проверок). Поллер
	// обновляет настройку checker_interval на каждом цикле в pollOnce.
	if v := p.st.GetSetting("checker_interval", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 10 {
			return n
		}
	}
	return p.cfg.PollInterval // фолбэк, если /api/v1/config недоступен
}

func (p *Poller) autocleanDefault() string {
	if p.cfg.StaleAfterHours > 0 {
		return "1"
	}
	return "0"
}

// Run запускает цикл опроса до отмены контекста.
func (p *Poller) Run(ctx context.Context) {
	first := true
	for {
		if err := p.pollOnce(ctx); err != nil {
			slog.Error("poll error", "err", err)
			if first {
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
				continue
			}
		}
		first = false
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(p.interval()) * time.Second):
		}
	}
}

// RunOnce — один немедленный опрос (кнопка «Обновить» и после смены подписки в
// боте). Сериализован с фоновым циклом через mu.
func (p *Poller) RunOnce(ctx context.Context) error { return p.pollOnce(ctx) }

func (p *Poller) pollOnce(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	proxies, err := p.client.Fetch(ctx)
	if err != nil {
		return err
	}
	// Синхронизируем такт со страницей: узнаём реальный интервал проверок чекера.
	if cc, cerr := p.client.FetchConfig(ctx); cerr == nil && cc.CheckInterval >= 10 {
		_ = p.st.SetSetting("checker_interval", strconv.Itoa(cc.CheckInterval))
	}
	now := time.Now()
	nowLocal := now.In(p.loc)
	nowUnix := now.Unix()
	today := nowLocal.Format("2006-01-02")
	cutoffDay := nowLocal.AddDate(0, 0, -(p.cfg.Days + 1)).Format("2006-01-02")

	// --- отсев глобального сбоя чекера (confirm-then-write) ---
	nValid, nOffline := 0, 0
	for _, px := range proxies {
		if px.StableID == "" {
			continue
		}
		nValid++
		if !px.Online {
			nOffline++
		}
	}
	allDown := nValid >= 2 && float64(nOffline)/float64(nValid) >= p.cfg.GlobalOutageRatio
	if allDown && p.st.GetSetting("skip_global", p.skipGlobalDefault()) == "1" {
		p.streak++
		if p.streak < 2 {
			slog.Warn("global-outage: всё офлайн разом — ждём подтверждения след. циклом",
				"offline", nOffline, "valid", nValid)
			p.emit(Event{Type: EventGlobalOutageSuspected, Offline: nOffline, Total: nValid})
			return nil
		}
		slog.Warn("global-outage подтверждён — пишем как реальный простой", "streak", p.streak)
		p.emit(Event{Type: EventGlobalOutageConfirmed, Offline: nOffline, Total: nValid})
	} else {
		if p.streak >= 2 {
			p.emit(Event{Type: EventGlobalOutageCleared, Offline: nOffline, Total: nValid})
		}
		p.streak = 0
	}

	autoclean := p.st.GetSetting("autoclean", p.autocleanDefault()) == "1"
	// Часы устаревания берём из настройки (правится из бота), дефолт — из env.
	staleHours := p.cfg.StaleAfterHours
	if v := p.st.GetSetting("stale_hours", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			staleHours = n
		}
	}

	// Транзиции статуса для алертов считаем ДО записи (по prev в current).
	transitions := p.detectTransitions(proxies, nowUnix)

	cleaned, err := p.st.PollWrite(proxies, store.PollWriteParams{
		Now:              nowUnix,
		Today:            today,
		PollInterval:     p.interval(),
		Autoclean:        autoclean,
		StaleAfterHours:  staleHours,
		CutoffDay:        cutoffDay,
		SampleRetainDays: p.cfg.SampleRetainDays,
	})
	if err != nil {
		return err
	}
	if len(cleaned) > 0 {
		slog.Info("auto-cleanup", "removed", len(cleaned))
	}
	maint, _ := p.st.MaintenanceNames(nowUnix)
	for _, t := range transitions {
		if maint[t.Name] {
			continue // сервер в обслуживании — алерты подавляем
		}
		p.emit(t)
	}
	for _, e := range p.detectPingEvents(proxies, p.st.PingThreshold(), maint) {
		p.emit(e)
	}
	return nil
}

// detectPingEvents отслеживает превышение порога пинга на уровне группы
// (репрезентативная задержка = минимум среди online-членов). Состояние хранится
// в p.pingHigh — алерт только на переходах в «высокий» и обратно (анти-флап).
func (p *Poller) detectPingEvents(proxies []checker.Proxy, threshold int, maint map[string]bool) []Event {
	if threshold <= 0 {
		if len(p.pingHigh) > 0 {
			p.pingHigh = map[string]bool{}
		}
		return nil
	}
	rep := map[string]int{}
	for _, px := range proxies {
		if px.StableID == "" || !px.Online || px.LatencyMs <= 0 {
			continue
		}
		name := px.Name
		if name == "" {
			name = px.StableID
		}
		if cur, ok := rep[name]; !ok || px.LatencyMs < cur {
			rep[name] = px.LatencyMs
		}
	}
	var out []Event
	newState := map[string]bool{}
	for name, lat := range rep {
		isHigh := lat > threshold
		newState[name] = isHigh
		if maint[name] {
			continue
		}
		if isHigh && !p.pingHigh[name] {
			out = append(out, Event{Type: EventHighPing, Name: name, Latency: lat, Online: true})
		} else if !isHigh && p.pingHigh[name] {
			out = append(out, Event{Type: EventPingOK, Name: name, Latency: lat, Online: true})
		}
	}
	p.pingHigh = newState
	return out
}
