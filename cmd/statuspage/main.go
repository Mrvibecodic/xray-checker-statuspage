// Command statuspage — Go-порт статуспейджа поверх xray-checker.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"xray-status/internal/bot"
	"xray-status/internal/checker"
	"xray-status/internal/config"
	"xray-status/internal/poller"
	"xray-status/internal/secret"
	"xray-status/internal/store"
	"xray-status/internal/web"
	"xray-status/internal/webctl"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()

	if dir := filepath.Dir(cfg.DBPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			slog.Error("mkdir data dir", "err", err)
			os.Exit(1)
		}
	}
	dbDSN := cfg.DBPath
	if cfg.DBDriver == "postgres" || cfg.DBDriver == "postgresql" || cfg.DBDriver == "pg" {
		dbDSN = cfg.DBDSN
	}
	st, err := store.Open(cfg.DBDriver, dbDSN)
	if err != nil {
		slog.Error("open store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	secretKey := cfg.SecretKey
	if secretKey == "" {
		// SECRET_KEY не задан — генерируем и храним ключ в томе данных.
		if k, err := secret.EnsureKeyFile(filepath.Join(filepath.Dir(cfg.DBPath), "secret.key")); err != nil {
			slog.Warn("auto secret key failed", "err", err)
		} else {
			secretKey = k
		}
	}
	if secretKey != "" {
		if err := st.EnableSecrets(secretKey); err != nil {
			slog.Warn("secrets disabled", "err", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Telegram-бот (control plane + алерты). nil, если BOT_TOKEN не задан.
	tb, err := bot.New(cfg, st)
	if err != nil {
		slog.Error("bot init", "err", err)
		os.Exit(1)
	}

	pl := poller.New(cfg, st, checker.New(cfg.CheckerURL))
	pl.OnEvent(func(e poller.Event) {
		slog.Info("event", "type", e.Type, "name", e.Name)
		if tb != nil {
			tb.HandleEvent(e)
		}
	})
	go pl.Run(ctx)
	if tb != nil {
		tb.SetPollNow(pl.RunOnce)
		go tb.Run(ctx)
		go tb.RunScheduler(ctx)
	}

	appHandler := web.New(cfg, st).Handler()

	// Публичный веб-сервер управляется рантайм-контроллером: бот может
	// запускать/останавливать прослушивание портов «по команде». На старте
	// поднимаем, если так сохранено (по умолчанию — да, чтобы работало сразу).
	webc := webctl.New(cfg, st, appHandler)
	if tb != nil {
		tb.AttachWeb(webc)
	}
	if webc.Enabled() {
		if err := webc.Start(); err != nil {
			slog.Error("web server start", "err", err)
		}
	} else {
		slog.Info("web server disabled (по сохранённому состоянию); включить — из бота")
	}

	// Внутренний сервер (только docker-сеть, порт не публикуется): отдаёт чекеру
	// отфильтрованную подписку по /sub.
	internalSrv := &http.Server{
		// Только loopback: при network_mode: host это закрывает 8081 от внешней
		// сети, но чекер (localhost:8081/sub) в той же host-сети до него достаёт.
		Addr:              "127.0.0.1:" + cfg.InternalPort,
		Handler:           web.NewInternal(cfg, st).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	go func() {
		slog.Info("internal listening", "addr", internalSrv.Addr)
		if err := internalSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("internal server error", "err", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down")
	cancel()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	webc.Shutdown(shutCtx)
	_ = internalSrv.Shutdown(shutCtx)
}
