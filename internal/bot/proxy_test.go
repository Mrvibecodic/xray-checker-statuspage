package bot

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestTelegramHTTPClientRejectsInvalidProxy(t *testing.T) {
	for _, rawURL := range []string{
		"http://127.0.0.1:1080",
		"socks5://",
		"socks5://127.0.0.1:1080/path",
		"socks5://127.0.0.1:bad",
	} {
		if _, err := newTelegramHTTPClient(rawURL); err == nil {
			t.Errorf("newTelegramHTTPClient(%q) succeeded", rawURL)
		}
	}
}

func TestTelegramHTTPClientUsesSOCKS5(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	connected := make(chan struct{}, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

		header := make([]byte, 2)
		if _, readErr := io.ReadFull(conn, header); readErr != nil || header[0] != 5 {
			return
		}
		methods := make([]byte, int(header[1]))
		if _, readErr := io.ReadFull(conn, methods); readErr != nil {
			return
		}
		connected <- struct{}{}
		_, _ = conn.Write([]byte{5, 0})
	}()

	client, err := newTelegramHTTPClient("socks5://" + listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_, _ = client.Get("https://api.telegram.org")

	select {
	case <-connected:
	case <-time.After(2 * time.Second):
		t.Fatal("HTTP client did not connect to the SOCKS5 proxy")
	}
}
