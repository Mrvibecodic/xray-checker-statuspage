package bot

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"

	xproxy "golang.org/x/net/proxy"
)

const telegramPollTimeout = time.Minute

// newTelegramHTTPClient ограничивает прокси только Telegram Bot API. Клиенты
// чекера, подписок и обновлений продолжают ходить напрямую.
func newTelegramHTTPClient(rawURL string) (*http.Client, error) {
	proxyURL, err := url.Parse(rawURL)
	if err != nil {
		// Ошибки url.Parse могут содержать исходную строку вместе с паролем.
		return nil, errors.New("TELEGRAM_PROXY содержит некорректный URL")
	}
	if proxyURL.Scheme != "socks5" && proxyURL.Scheme != "socks5h" {
		return nil, errors.New("TELEGRAM_PROXY должен использовать схему socks5:// или socks5h://")
	}
	if proxyURL.Hostname() == "" {
		return nil, errors.New("TELEGRAM_PROXY должен содержать адрес прокси")
	}
	if proxyURL.Path != "" || proxyURL.RawQuery != "" || proxyURL.Fragment != "" {
		return nil, errors.New("TELEGRAM_PROXY не должен содержать путь, параметры или fragment")
	}

	forward := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	dialer, err := xproxy.FromURL(proxyURL, forward)
	if err != nil {
		return nil, errors.New("не удалось настроить TELEGRAM_PROXY")
	}
	contextDialer, ok := dialer.(xproxy.ContextDialer)
	if !ok {
		return nil, errors.New("TELEGRAM_PROXY не поддерживает отмену соединений")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = contextDialer.DialContext
	return &http.Client{Transport: transport, Timeout: telegramPollTimeout}, nil
}
