package bot

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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
