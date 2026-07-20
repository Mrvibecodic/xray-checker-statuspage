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
// публикуется наружу). Отдаёт чекеру объединённую подписку по /sub: несколько
// upstream-подписок склеиваются в один список/JSON (ПЛАН §8.2, §13).
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
	// UA чекера форвардим апстриму: в json-режиме чекер ходит под "Happ/1.0", и
	// Remnawave отдаёт XRAY_JSON (с балансерами/роутингом); иначе — обычный
	// формат (base64 share-ссылки). Пустой UA => дефолт Go => прежнее поведение.
	ua := r.Header.Get("User-Agent")
	var raws [][]byte
	for _, u := range urls {
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		if ua != "" {
			req.Header.Set("User-Agent", ua)
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
	// XRAY_JSON (Remnawave под Happ): склеиваем конфиги на уровне JSON и отдаём
	// как есть — иначе балансеры/роутинг терялись бы при построчном merge.
	// Не-JSON (base64/plaintext share-ссылки) идут прежним путём.
	if out, ok := sub.FilterJSON(raws); ok {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(out)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(sub.Merge(raws))
}
