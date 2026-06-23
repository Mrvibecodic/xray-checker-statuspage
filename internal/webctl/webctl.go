// Package webctl — рантайм-управление публичным веб-сервером статуспейджа:
// бот может запускать/останавливать прослушивание портов «по команде»,
// не перезапуская процесс. Состояние (вкл/выкл) сохраняется в настройках,
// чтобы переживать рестарт.
package webctl

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"xray-status/internal/config"
	"xray-status/internal/store"
)

const settingKey = "web_enabled"

type Controller struct {
	cfg     config.Config
	st      *store.Store
	handler http.Handler

	mu      sync.Mutex
	servers []*http.Server
	running bool

	probeAt time.Time
	probeOK bool
}

func New(cfg config.Config, st *store.Store, handler http.Handler) *Controller {
	return &Controller{cfg: cfg, st: st, handler: handler}
}

// Enabled — должен ли веб-сервер быть поднят (по сохранённому состоянию).
// По умолчанию (на первом запуске) — да, чтобы страница работала из коробки.
func (c *Controller) Enabled() bool {
	return c.st.GetSetting(settingKey, "1") != "0"
}

func (c *Controller) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

func (c *Controller) domain() string {
	if c.cfg.Domain != "" {
		return c.cfg.Domain
	}
	return c.st.PublicDomain()
}

// Start поднимает прослушивание портов. Если уже запущен — перезапускает
// (чтобы подхватить новый домен). Возвращает ошибку, если основной HTTP-порт
// занять не удалось.
func (c *Controller) Start() error {
	_ = c.Stop() // идемпотентно: сначала гасим прежние слушатели
	c.mu.Lock()
	defer c.mu.Unlock()

	var servers []*http.Server

	// основной HTTP
	httpSrv := &http.Server{
		Addr:              "0.0.0.0:" + c.cfg.Port,
		Handler:           c.handler,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := listen(httpSrv); err != nil {
		return fmt.Errorf("порт %s занять не удалось: %w", c.cfg.Port, err)
	}
	servers = append(servers, httpSrv)
	slog.Info("web: HTTP up", "addr", httpSrv.Addr)

	// доп. :8080 (совместимость/CI) — не критичен
	if c.cfg.Port != "8080" {
		alt := &http.Server{Addr: "0.0.0.0:8080", Handler: c.handler, ReadHeaderTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second}
		if err := listen(alt); err != nil {
			slog.Warn("web: alt :8080", "err", err)
		} else {
			servers = append(servers, alt)
		}
	}

	// HTTPS, если задан домен — не критичен (HTTP уже живёт). Источник серта
	// выбирается по TLS_MODE/CERT_FILE (см. tlsConfigFor): по умолчанию
	// self-signed — за Cloudflare (SSL=Full) этого достаточно, 525 уходит.
	if d := c.domain(); d != "" && c.cfg.TLSMode != "off" {
		tlsCfg, mode, err := c.tlsConfigFor(d)
		if err != nil {
			slog.Warn("web: HTTPS cert", "err", err, "mode", mode)
		} else {
			httpsSrv := &http.Server{
				Addr:              "0.0.0.0:" + c.cfg.HTTPSPort,
				Handler:           c.handler,
				TLSConfig:         tlsCfg,
				ReadHeaderTimeout: 10 * time.Second,
				WriteTimeout:      30 * time.Second,
				IdleTimeout:       60 * time.Second,
			}
			if err := listenTLS(httpsSrv); err != nil {
				slog.Warn("web: HTTPS", "err", err)
			} else {
				servers = append(servers, httpsSrv)
				slog.Info("web: HTTPS up", "domain", d, "addr", httpsSrv.Addr, "mode", mode)
			}
		}
	}

	c.servers = servers
	c.running = true
	_ = c.st.SetSetting(settingKey, "1")
	return nil
}

// Stop гасит публичные слушатели и помечает веб как выключенный (сохраняется).
func (c *Controller) Stop() error {
	c.mu.Lock()
	srvs := c.servers
	c.servers = nil
	c.running = false
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, s := range srvs {
		_ = s.Shutdown(ctx)
	}
	_ = c.st.SetSetting(settingKey, "0")
	return nil
}

// Shutdown гасит слушатели при выходе процесса, НЕ меняя сохранённое состояние.
func (c *Controller) Shutdown(ctx context.Context) {
	c.mu.Lock()
	srvs := c.servers
	c.mu.Unlock()
	for _, s := range srvs {
		_ = s.Shutdown(ctx)
	}
}

// Probe проверяет доступность локально: бьёт по 127.0.0.1:<port>/healthz и
// считает «работает», если статус < 500. Внешнюю проверку по https://домен
// сознательно НЕ делаем приговором — за Cloudflare и при hairpin-NAT self-probe
// часто врёт, хотя страница реально открывается. Результат кэшируется на 5с,
// чтобы не тормозить отрисовку меню.
func (c *Controller) Probe() bool {
	c.mu.Lock()
	if time.Since(c.probeAt) < 5*time.Second {
		ok := c.probeOK
		c.mu.Unlock()
		return ok
	}
	c.mu.Unlock()

	// Авторитетная проверка — локальный порт: «сервер реально отдаёт страницу».
	// Внешнюю проверку по https://домен НЕ используем как приговор: за Cloudflare
	// и при hairpin-NAT self-probe часто врёт (страница при этом работает).
	cl := &http.Client{Timeout: 3 * time.Second}
	ok := false
	if resp, err := cl.Get("http://127.0.0.1:" + c.cfg.Port + "/healthz"); err == nil {
		_ = resp.Body.Close()
		ok = resp.StatusCode < 500
	}

	c.mu.Lock()
	c.probeAt = time.Now()
	c.probeOK = ok
	c.mu.Unlock()
	return ok
}

// Status — короткое описание для бота.
func (c *Controller) Status() string {
	if !c.Running() {
		return "остановлен"
	}
	if d := c.domain(); d != "" {
		return "работает: https://" + d + " (порты 80/443)"
	}
	return "работает: http (порт " + c.cfg.Port + ")"
}

// tlsConfigFor выбирает источник TLS-сертификата для домена d.
//
//	TLS_MODE=letsencrypt        — autocert (Let's Encrypt). Только для origin
//	                              БЕЗ Cloudflare-прокси: за CF ACME-валидация
//	                              (TLS-ALPN-01) не проходит и серт не выдаётся.
//	CERT_FILE+KEY_FILE заданы   — готовый серт (Cloudflare Origin Certificate);
//	                              работает и в режиме CF Full(strict).
//	иначе (по умолчанию)        — self-signed; за CF SSL=Full рукопожатие
//	                              завершается, ошибка 525 исчезает.
func (c *Controller) tlsConfigFor(d string) (*tls.Config, string, error) {
	mode := c.cfg.TLSMode
	if mode == "" {
		if c.cfg.CertFile != "" && c.cfg.KeyFile != "" {
			mode = "file"
		} else {
			mode = "selfsigned"
		}
	}
	switch mode {
	case "letsencrypt":
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      autocert.DirCache(filepath.Join(filepath.Dir(c.cfg.DBPath), "certs")),
			HostPolicy: autocert.HostWhitelist(d),
		}
		return m.TLSConfig(), mode, nil
	case "file":
		cert, err := tls.LoadX509KeyPair(c.cfg.CertFile, c.cfg.KeyFile)
		if err != nil {
			return nil, mode, err
		}
		return &tls.Config{Certificates: []tls.Certificate{cert}}, mode, nil
	default: // selfsigned
		// Кэшируем серт в томе данных, чтобы НЕ генерировать новый на каждый
		// старт/перезапуск веба (и переживать рестарт процесса).
		cert, err := loadOrCreateSelfSigned(filepath.Join(filepath.Dir(c.cfg.DBPath), "certs"), d)
		if err != nil {
			return nil, "selfsigned", err
		}
		return &tls.Config{Certificates: []tls.Certificate{cert}}, "selfsigned", nil
	}
}

// listen биндит порт синхронно (чтобы поймать ошибку «занят»), затем обслуживает в фоне.
func listen(s *http.Server) error {
	ln, err := netListen(s.Addr)
	if err != nil {
		return err
	}
	go func() { _ = s.Serve(ln) }()
	return nil
}

func listenTLS(s *http.Server) error {
	ln, err := netListen(s.Addr)
	if err != nil {
		return err
	}
	go func() { _ = s.ServeTLS(ln, "", "") }()
	return nil
}
