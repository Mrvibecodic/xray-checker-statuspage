package web

import (
	"io"
	"net/http"
	"time"

	"xray-status/internal/config"
	"xray-status/internal/store"
	"xray-status/internal/sub"
)

// Internal — отдельный HTTP-сервер ТОЛЬКО для внутренней docker-сети (порт не
// публикуется наружу). Отдаёт чекеру отфильтрованную подписку по /sub: серверы,
// выключенные через бота, в неё не попадают (ПЛАН §8.2, §13).
type Internal struct {
	cfg    config.Config
	st     *store.Store
	client *http.Client
}

func NewInternal(cfg config.Config, st *store.Store) *Internal {
	return &Internal{cfg: cfg, st: st, client: &http.Client{Timeout: 15 * time.Second}}
}

func (in *Internal) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/sub", in.sub)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})
	return mux
}

func (in *Internal) sub(w http.ResponseWriter, r *http.Request) {
	// Источник списка: заданные из бота подписки; если их нет — фолбэк на env
	// (SUBSCRIPTION_URL, допускает список через запятую).
	urls := in.st.SubscriptionURLs()
	if len(urls) == 0 && in.cfg.SubscriptionURL != "" {
		urls = sub.ParseURLs(in.cfg.SubscriptionURL)
	}
	if len(urls) == 0 {
		http.Error(w, "subscription not configured", http.StatusNotFound)
		return
	}
	// Тянем каждую подписку; недоступную пропускаем, чтобы одна сбойная не
	// обрушила весь список. 502 — только если не удалось получить ни одной.
	var raws [][]byte
	for _, u := range urls {
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		resp, err := in.client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		resp.Body.Close()
		raws = append(raws, body)
	}
	if len(raws) == 0 {
		http.Error(w, "upstream fetch failed", http.StatusBadGateway)
		return
	}
	disabled, _ := in.st.DisabledServers()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(sub.Merge(raws, disabled))
}
