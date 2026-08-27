// Package web поднимает публичный (read-only) HTTP-сервер: отдаёт вшитый
// фронтенд (байт-в-байт перенос текущей страницы) и API.
//
// Мутирующих эндпоинтов на этом порту нет и не будет — весь контроль уходит в
// бот (ПЛАН §1, §11).
package web

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"xray-status/internal/config"
	"xray-status/internal/store"
	"xray-status/internal/summary"
)

//go:embed assets/index.html.tpl assets/index2.html.tpl assets/index3.html.tpl assets/logo.svg assets/flags
var assets embed.FS

// uniqTokens — те же CSS/JS-токены, что рандомизирует Python-версия для
// анти-фингерпринта (app.py: _UNIQ_TOKENS). Каждый токен получает СВОЁ случайное
// имя на каждый запрос — между копиями нет общего префикса.
var uniqTokens = []string{
	// уникальный для продукта вокабуляр классов/id — рандомизируется на КАЖДУЮ
	// загрузку, чтобы у массовых копий не было общего CSS/HTML-отпечатка.
	// Список выверен: ни один токен не пересекается с CSS-свойствами, JSON-ключами
	// (/api/summary) или критичными JS-идентификаторами (см. внутренний аудит).
	"tchartwrap", "tcaption", "tchart", "tcanvas", "tscroll", "tstats", "taxis", "tyaxis",
	"inc-tlrow", "inc-card", "inc-title", "inc-status", "inc-time", "inc-sev", "inc-aff",
	"inc-tlt", "inc-tls", "inc-tl", "inc-h",
	"mnt-badge", "mnt-line", "maint",
	"s-online", "s-uptime", "s-ping", "s-fresh",
	"section-head", "pulseClaudeDark", "pulseClaude",
	"stat2", "sdot", "phead", "pgrad", "overall", "topr", "vsub",
	"item", "row", "panel", "flag", "chev", "bars", "brand", "logo",
	"legend", "empty", "skel", "pill", "open", "stat", "nm", "dot",
	"tip", "list", "foot",
}

const (
	noCache     = "no-cache, no-store, must-revalidate"
	staticCache = "public, max-age=31536000, immutable"
)

type Server struct {
	cfg   config.Config
	st    *store.Store
	page  []byte
	page2 []byte
	page3 []byte
	logo  []byte
}

// New собирает страницу один раз на старте: применяет анти-фингерпринт и
// подставляет плейсхолдеры (__TITLE__/__SUBTITLE__/__DAYS__/__LOGO__).
func New(cfg config.Config, st *store.Store) *Server {
	tplBytes, err := assets.ReadFile("assets/index.html.tpl")
	if err != nil {
		panic("embed: index.html.tpl: " + err.Error())
	}
	tpl2Bytes, err := assets.ReadFile("assets/index2.html.tpl")
	if err != nil {
		panic("embed: index2.html.tpl: " + err.Error())
	}
	tpl3Bytes, err := assets.ReadFile("assets/index3.html.tpl")
	if err != nil {
		panic("embed: index3.html.tpl: " + err.Error())
	}

	// Дефолтный фавикон/логотип рандомизируется на КАЖДЫЙ старт инстанса — у
	// массовых копий нет одинакового дефолтного фавикона (анти-фингерпринт).
	logo := []byte(randomFaviconSVG())

	// Только статичная подстановка дней. Анти-фингерпринт (uniquify), рандомизация
	// темы и остальные плейсхолдеры применяются ПО-ЗАПРОСНО в index().
	days := strconv.Itoa(cfg.Days)
	raw := strings.ReplaceAll(string(tplBytes), "__DAYS__", days)
	raw2 := strings.ReplaceAll(string(tpl2Bytes), "__DAYS__", days)
	raw3 := strings.ReplaceAll(string(tpl3Bytes), "__DAYS__", days)
	return &Server{cfg: cfg, st: st, page: []byte(raw), page2: []byte(raw2), page3: []byte(raw3), logo: logo}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)
	mux.HandleFunc("/api/summary", s.summary)
	mux.HandleFunc("/api/today", s.today)
	mux.HandleFunc("/api/day", s.day)
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/logo", s.logoHandler)
	mux.HandleFunc("/flags/", s.flagHandler)
	mux.HandleFunc("/favicon.ico", s.faviconHandler)
	mux.HandleFunc("/favicon.png", s.faviconHandler)
	return s.baseHeaders(mux)
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		s.send(w, r, http.StatusNotFound, []byte("Not Found"), "text/plain; charset=utf-8", noCache)
		return
	}
	theme := s.st.GetSetting("theme", "dark")
	switch theme {
	case "light", "dark", "claude", "claude-dark", "v2", "minimal":
	default:
		theme = "dark"
	}
	title := s.st.GetSetting("title", s.cfg.Title)
	subtitle := s.st.GetSetting("subtitle", s.cfg.Subtitle)
	desc := s.st.GetSetting("description", s.cfg.Description)
	// фавикон: и как /favicon.ico, и как логотип слева от названия
	favTag := `<link rel="icon" href="favicon.ico">`
	logoHTML := string(s.logo)
	if _, data, ok := s.st.GetAsset("favicon"); ok {
		v := strconv.Itoa(len(data))
		favTag = `<link rel="icon" href="favicon.ico?v=` + v + `">`
		logoHTML = `<img src="favicon.ico?v=` + v + `" alt="">`
	}

	// «Тема 2.0» (v2) — отдельный макет из другого шаблона; light/dark — базовый.
	src := s.page
	switch theme {
	case "v2":
		src = s.page2
	case "minimal":
		src = s.page3
	}
	// АНТИ-ФИНГЕРПРИНТ: независимые случайные имена классов/id на каждый запрос,
	// плюс рандомизация имени атрибута темы и его значений (нет константного
	// html[data-theme="..."]-отпечатка между копиями).
	page := randomizeTheme(uniquify(string(src), uniqTokens), theme)
	rep := strings.NewReplacer(
		"__TITLE__", html.EscapeString(title),
		"__SUBTITLE__", html.EscapeString(subtitle),
		"__DESC__", html.EscapeString(desc),
		"__FAVICON__", favTag,
		"__LOGO__", logoHTML,
	)
	out := injectNoise(rep.Replace(page))
	s.send(w, r, http.StatusOK, []byte(out), "text/html; charset=utf-8", noCache)
}

// injectNoise добавляет безвредный случайный «шум», чтобы байтовый/структурный
// хеш каждой выдачи отличался. ПРАВИЛО: в выдаваемую ПОСЕТИТЕЛЮ страницу НЕ
// добавляем HTML/CSS/JS-комментарии (они видны в исходнике) — энтропию даёт
// случайный <meta> и переменный хвост из пробелов/переводов строк.
func injectNoise(page string) string {
	page = strings.Replace(page, "<head>", "<head>\n<meta name=\""+randHex(4)+"\" content=\""+randHex(6+randN(10))+"\">", 1)
	return page + strings.Repeat("\n", 1+randN(3)) + strings.Repeat(" ", randN(9))
}

// randHex — n случайных hex-символов.
func randHex(n int) string {
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("0", n)
	}
	return hex.EncodeToString(b)[:n]
}

// randN — случайное число [0,max).
func randN(max int) int {
	if max <= 0 {
		return 0
	}
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0
	}
	return (int(b[0])<<8 | int(b[1])) % max
}

// faviconHandler отдаёт загруженный из бота фавикон, иначе встроенный логотип.
func (s *Server) faviconHandler(w http.ResponseWriter, r *http.Request) {
	if mime, data, ok := s.st.GetAsset("favicon"); ok {
		s.send(w, r, http.StatusOK, data, mime, "public, max-age=300")
		return
	}
	s.send(w, r, http.StatusOK, s.logo, "image/svg+xml", staticCache)
}

// summary — публичный (admin=false): скрытые/отсутствующие серверы не отдаём.
func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	payload, err := summary.BuildSummary(s.st, s.cfg, false)
	if err != nil {
		slog.Error("build summary", "err", err)
		s.send(w, r, http.StatusInternalServerError, []byte("{}"), "application/json; charset=utf-8", noCache)
		return
	}
	s.sendJSON(w, r, payload)
}

func (s *Server) today(w http.ResponseWriter, r *http.Request) {
	payload, err := summary.BuildToday(s.st, s.cfg, qparam(r, "sid"))
	if err != nil {
		slog.Error("build today", "err", err)
		s.send(w, r, http.StatusInternalServerError, []byte("{}"), "application/json; charset=utf-8", noCache)
		return
	}
	s.sendJSON(w, r, payload)
}

func (s *Server) day(w http.ResponseWriter, r *http.Request) {
	payload, err := summary.BuildDay(s.st, s.cfg, qparam(r, "sid"), qparam(r, "date"))
	if err != nil {
		slog.Error("build day", "err", err)
		s.send(w, r, http.StatusInternalServerError, []byte("{}"), "application/json; charset=utf-8", noCache)
		return
	}
	s.sendJSON(w, r, payload)
}

func qparam(r *http.Request, k string) string {
	v, _ := url.QueryUnescape(r.URL.Query().Get(k))
	return v
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	s.send(w, r, http.StatusOK, []byte("OK"), "text/plain; charset=utf-8", noCache)
}

func (s *Server) logoHandler(w http.ResponseWriter, r *http.Request) {
	s.send(w, r, http.StatusOK, s.logo, "image/svg+xml", staticCache)
}

// flagHandler отдаёт вшитый SVG-флаг по коду страны (/flags/<cc>.svg) — флаги
// self-hosted в бинаре, без обращения к внешнему CDN (работает за блокировками
// и не создаёт внешний фингерпринт).
func (s *Server) flagHandler(w http.ResponseWriter, r *http.Request) {
	cc := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/flags/"), ".svg")
	if len(cc) != 2 || cc[0] < 'a' || cc[0] > 'z' || cc[1] < 'a' || cc[1] > 'z' {
		s.send(w, r, http.StatusNotFound, []byte("Not Found"), "text/plain; charset=utf-8", noCache)
		return
	}
	data, err := assets.ReadFile("assets/flags/" + cc + ".svg")
	if err != nil {
		s.send(w, r, http.StatusNotFound, []byte("Not Found"), "text/plain; charset=utf-8", noCache)
		return
	}
	s.send(w, r, http.StatusOK, data, "image/svg+xml", staticCache)
}

func (s *Server) sendJSON(w http.ResponseWriter, r *http.Request, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		s.send(w, r, http.StatusInternalServerError, []byte("{}"), "application/json; charset=utf-8", noCache)
		return
	}
	s.send(w, r, http.StatusOK, b, "application/json; charset=utf-8", noCache)
}

// send пишет ответ с маскировкой Server-заголовка, noindex и gzip (как app.py).
func (s *Server) send(w http.ResponseWriter, r *http.Request, code int, body []byte, ctype, cache string) {
	h := w.Header()
	h.Set("Content-Type", ctype)
	h.Set("Server", s.cfg.ServerHeader)
	h.Set("X-Robots-Tag", "noindex, nofollow")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", cache)

	compressible := strings.Contains(ctype, "text/") || strings.Contains(ctype, "json") ||
		strings.Contains(ctype, "javascript") || strings.Contains(ctype, "svg")
	if compressible && len(body) >= 256 && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		var buf bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&buf, 6)
		_, _ = zw.Write(body)
		_ = zw.Close()
		body = buf.Bytes()
		h.Set("Content-Encoding", "gzip")
		h.Set("Vary", "Accept-Encoding")
	}
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(code)
	_, _ = w.Write(body)
}

func (s *Server) baseHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", s.cfg.ServerHeader)
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		next.ServeHTTP(w, r)
	})
}

func uniqName() string {
	// случайная буква-начало + случайная длина hex — валидный CSS/JS-идентификатор
	// случайной длины (формат имени не константа между загрузками).
	letters := "abcdefghijklmnopqrstuvwxyz"
	var lb [1]byte
	_, _ = rand.Read(lb[:])
	n := 4 + randN(6) // 4..9 hex-символов
	return string(letters[int(lb[0])%len(letters)]) + randHex(n)
}

// randomizeTheme рандомизирует ИМЯ атрибута темы (data-theme -> data-XXXX) и
// его ЗНАЧЕНИЯ (light/dark -> случайные), консистентно в CSS-селекторах и в теге
// <html>. Между копиями нет константного `html[data-theme="dark"]`-отпечатка.
// Для «темы 2.0» (v2) значение тега не совпадает ни с одним явным селектором —
// страница падает на :root + @media(prefers-color-scheme) => авто light/dark.
func randomizeTheme(page, theme string) string {
	attr := "data-" + randHex(3+randN(3))
	v := map[string]string{
		"light":       uniqName(),
		"dark":        uniqName(),
		"claude":      uniqName(),
		"claude-dark": uniqName(),
	}
	vSel, ok := v[theme]
	if !ok { // v2 -> авто (системная light/dark): не совпадает ни с одним селектором
		vSel = uniqName()
	}
	return strings.NewReplacer(
		`data-theme="light"`, attr+`="`+v["light"]+`"`,
		`data-theme="dark"`, attr+`="`+v["dark"]+`"`,
		`data-theme="claude-dark"`, attr+`="`+v["claude-dark"]+`"`,
		`data-theme="claude"`, attr+`="`+v["claude"]+`"`,
		`data-theme="__THEME__"`, attr+`="`+vSel+`"`,
	).Replace(page)
}

// randomFaviconSVG генерирует уникальный дефолтный фавикон/логотип (на старт
// инстанса) — простой значок со случайным оттенком/радиусом, чтобы у массовых
// копий не было одинакового дефолтного фавикона.
func randomFaviconSVG() string {
	hue := randN(360)
	hue2 := (hue + 24 + randN(72)) % 360
	rx := 8 + randN(10)
	dot := 8 + randN(7)
	gid := uniqName()
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">`+
			`<defs><linearGradient id="%s" x1="0" y1="0" x2="1" y2="1">`+
			`<stop offset="0" stop-color="hsl(%d,68%%,53%%)"/>`+
			`<stop offset="1" stop-color="hsl(%d,66%%,43%%)"/></linearGradient></defs>`+
			`<rect x="2" y="2" width="60" height="60" rx="%d" fill="url(#%s)"/>`+
			`<circle cx="32" cy="32" r="%d" fill="#fff" fill-opacity="0.92"/></svg>`,
		gid, hue, hue2, rx, gid, dot,
	)
}

// uniquify заменяет каждый токен на СОБСТВЕННОЕ случайное имя (а не общий
// префикс) — чтобы между копиями не было ни общего префикса, ни узнаваемых
// суффиксов классов. Замена только на границе [^\w-] с обеих сторон (не задевает
// подстроки), от длинных токенов к коротким. Множество заменяемых позиций такое
// же, как раньше; меняется лишь целевая строка, поэтому рендер не ломается.
func uniquify(s string, tokens []string) string {
	toks := append([]string(nil), tokens...)
	sort.Slice(toks, func(i, j int) bool { return len(toks[i]) > len(toks[j]) })
	repl := make(map[string]string, len(toks))
	used := make(map[string]bool, len(toks))
	for _, t := range toks {
		var name string
		for {
			name = uniqName()
			if !used[name] {
				break
			}
		}
		used[name] = true
		repl[t] = name
	}
	isWord := func(b byte) bool {
		return b == '_' || b == '-' ||
			(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
	}
	for _, t := range toks {
		r := repl[t]
		var b strings.Builder
		b.Grow(len(s) + 16)
		for i := 0; i < len(s); {
			if strings.HasPrefix(s[i:], t) {
				before := i == 0 || !isWord(s[i-1])
				after := i+len(t) >= len(s) || !isWord(s[i+len(t)])
				if before && after {
					b.WriteString(r)
					i += len(t)
					continue
				}
			}
			b.WriteByte(s[i])
			i++
		}
		s = b.String()
	}
	return s
}
