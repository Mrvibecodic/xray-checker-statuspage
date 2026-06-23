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
	url := in.cfg.SubscriptionURL // дефолт из env (та же, что у чекера)
	if in.st.HasSubscriptionURL() {
		if u, err := in.st.SubscriptionURL(); err == nil {
			url = u // override, заданный из бота
		}
	}
	if url == "" {
		http.Error(w, "subscription not configured", http.StatusNotFound)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		http.Error(w, "bad subscription url", http.StatusInternalServerError)
		return
	}
	resp, err := in.client.Do(req)
	if err != nil {
		http.Error(w, "upstream fetch failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	disabled, _ := in.st.DisabledServers()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(sub.Filter(body, disabled))
}
