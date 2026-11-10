package bot

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"xray-status/internal/config"
	"xray-status/internal/store"
	"xray-status/internal/storetest"
)

// socks5Relay — минимальный SOCKS5 (без авторизации, только CONNECT): считает
// подключения и проксирует их на запрошенный адрес.
func socks5Relay(t *testing.T) (addr string, connects *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	connects = &atomic.Int32{}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				head := make([]byte, 2)
				if _, err := io.ReadFull(conn, head); err != nil || head[0] != 5 {
					return
				}
				if _, err := io.ReadFull(conn, make([]byte, int(head[1]))); err != nil {
					return
				}
				_, _ = conn.Write([]byte{5, 0})
				req := make([]byte, 4)
				if _, err := io.ReadFull(conn, req); err != nil || req[1] != 1 {
					return
				}
				var host string
				switch req[3] {
				case 1:
					ip := make([]byte, 4)
					if _, err := io.ReadFull(conn, ip); err != nil {
						return
					}
					host = net.IP(ip).String()
				case 4:
					ip := make([]byte, 16)
					if _, err := io.ReadFull(conn, ip); err != nil {
						return
					}
					host = net.IP(ip).String()
				case 3:
					l := make([]byte, 1)
					if _, err := io.ReadFull(conn, l); err != nil {
						return
					}
					name := make([]byte, int(l[0]))
					if _, err := io.ReadFull(conn, name); err != nil {
						return
					}
					host = string(name)
				default:
					return
				}
				port := make([]byte, 2)
				if _, err := io.ReadFull(conn, port); err != nil {
					return
				}
				target, err := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(port)))))
				if err != nil {
					return
				}
				defer target.Close()
				connects.Add(1)
				_, _ = conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
				go func() { _, _ = io.Copy(target, conn) }()
				_, _ = io.Copy(conn, target)
			}()
		}
	}()
	return ln.Addr().String(), connects
}

// Файлы Telegram должны скачиваться тем же клиентом, что ходит в Bot API:
// иначе при TELEGRAM_PROXY загрузка фавикона идёт мимо прокси.
func TestFileDownloadUsesTelegramProxy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("PNGDATA"))
	}))
	defer srv.Close()
	proxyAddr, connects := socks5Relay(t)

	client, err := newTelegramHTTPClient("socks5://" + proxyAddr)
	if err != nil {
		t.Fatal(err)
	}
	tb := &Bot{tgHTTP: client}
	data, err := downloadLimited(context.Background(), tb.fileHTTPClient(), srv.URL+"/file/bot1:T/a.png", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "PNGDATA" {
		t.Fatalf("unexpected body: %q", data)
	}
	if connects.Load() == 0 {
		t.Fatal("file download bypassed the SOCKS5 proxy")
	}

	if (&Bot{}).fileHTTPClient() != directFileHTTP {
		t.Fatal("without TELEGRAM_PROXY the direct client must be used")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Ссылка на файл содержит токен бота — в тексте ошибки (он уходит в чат) её быть
// не должно; ответ не 200 — ошибка, а не «фавикон» из тела ответа.
func TestDownloadLimitedErrors(t *testing.T) {
	const token = "123456:SECRET"
	failing := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("TLS handshake timeout")
	})}
	_, err := downloadLimited(context.Background(), failing, "https://api.telegram.org/file/bot"+token+"/a.png", 1<<20)
	if err == nil {
		t.Fatal("expected a transport error")
	}
	if strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), "/file/") {
		t.Fatalf("error leaks the file URL: %v", err)
	}
	if !strings.Contains(err.Error(), "TLS handshake timeout") {
		t.Fatalf("error lost its cause: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"ok":false,"error_code":404}`, http.StatusNotFound)
	}))
	defer srv.Close()
	if data, err := downloadLimited(context.Background(), directFileHTTP, srv.URL+"/file/bot"+token+"/a.png", 1<<20); err == nil {
		t.Fatalf("expected an error for HTTP 404, got %q", data)
	}

	if _, err := downloadLimited(context.Background(), directFileHTTP, "http://bad host/"+token, 1<<20); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("bad link error must not echo the link: %v", err)
	}
}

// Файл больше лимита отклоняется целиком: обрезанная картинка — битый фавикон.
func TestDownloadLimitedRejectsOversized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.URL.Query().Get("n"))
		_, _ = w.Write(bytes.Repeat([]byte{'x'}, n))
	}))
	defer srv.Close()

	data, err := downloadLimited(context.Background(), directFileHTTP, srv.URL+"/?n=8", 8)
	if err != nil || len(data) != 8 {
		t.Fatalf("file of exactly max bytes must pass: len=%d err=%v", len(data), err)
	}
	if data, err := downloadLimited(context.Background(), directFileHTTP, srv.URL+"/?n=9", 8); !errors.Is(err, errFileTooLarge) {
		t.Fatalf("expected errFileTooLarge, got len=%d err=%v", len(data), err)
	}
}

// fakeTelegram — заглушка Bot API: считает вызовы методов, запоминает текст
// последнего сообщения и отдаёт файл заданного размера.
type fakeTelegram struct {
	mu       sync.Mutex
	calls    map[string]int
	lastText string
	fileBody []byte
}

func (f *fakeTelegram) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/file/") {
		f.mu.Lock()
		f.calls["download"]++
		body := f.fileBody
		f.mu.Unlock()
		_, _ = w.Write(body)
		return
	}
	method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	f.mu.Lock()
	f.calls[method]++
	if txt := r.FormValue("text"); txt != "" {
		f.lastText = txt
	}
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch method {
	case "deleteMessage":
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	case "getFile":
		_, _ = w.Write([]byte(`{"ok":true,"result":{"file_id":"f","file_path":"documents/fav.png"}}`))
	default:
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":7}}`))
	}
}

func TestFaviconUploadSizeLimit(t *testing.T) {
	cases := []struct {
		name      string
		declared  int64 // размер из сообщения Telegram
		body      int   // сколько байт реально отдаёт сервер файлов
		saved     bool
		getFile   int
		download  int
		wantInMsg string
	}{
		{"в пределах лимита", 4, 4, true, 1, 1, "Фавикон обновлён"},
		{"заявленный размер больше лимита", faviconMaxBytes + 1, 4, false, 0, 0, "больше 1 МБ"},
		{"реальный размер больше лимита", 0, faviconMaxBytes + 1, false, 1, 1, "больше 1 МБ"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, err := store.Open(storetest.DSN(t))
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			tg := &fakeTelegram{calls: map[string]int{}, fileBody: bytes.Repeat([]byte{'x'}, tc.body)}
			srv := httptest.NewServer(tg)
			defer srv.Close()
			b, err := bot.New("1:T", bot.WithServerURL(srv.URL), bot.WithSkipGetMe())
			if err != nil {
				t.Fatal(err)
			}
			tb := &Bot{b: b, st: st, cfg: config.Config{TZ: "UTC"}, admins: map[int64]bool{1: true}}
			_ = st.SetBotState(1, "await", "favicon")

			tb.handleFaviconUpload(context.Background(), 1, &models.Message{
				ID:       5,
				Document: &models.Document{FileID: "f", MimeType: "image/png", FileSize: tc.declared},
			})

			if _, data, ok := st.GetAsset("favicon"); ok != tc.saved {
				t.Fatalf("favicon saved=%v (%d bytes), want saved=%v", ok, len(data), tc.saved)
			}
			tg.mu.Lock()
			defer tg.mu.Unlock()
			if tg.calls["getFile"] != tc.getFile || tg.calls["download"] != tc.download {
				t.Fatalf("getFile=%d download=%d, want %d/%d", tg.calls["getFile"], tg.calls["download"], tc.getFile, tc.download)
			}
			if !strings.Contains(tg.lastText, tc.wantInMsg) {
				t.Fatalf("reply %q does not contain %q", tg.lastText, tc.wantInMsg)
			}
		})
	}
}
