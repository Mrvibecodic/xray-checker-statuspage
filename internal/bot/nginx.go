package bot

import (
	"fmt"

	"xray-status/internal/config"
	"xray-status/internal/store"
)

// cmdNginx выдаёт готовый конфиг reverse-proxy nginx для публикации статуспейджа
// под доменом (из настроек) и инструкцию по TLS.
func cmdNginx(st *store.Store, cfg config.Config) string {
	dom := st.PublicDomain()
	if dom == "" {
		dom = "status.example.com"
	}
	port := cfg.Port
	conf := fmt.Sprintf(`server {
    listen 80;
    server_name %s;
    location / {
        proxy_pass http://127.0.0.1:%s;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}`, dom, port)
	return "<b>nginx reverse-proxy</b>\n" +
		"1) Сохрани в /etc/nginx/sites-available/statuspage и сделай symlink в sites-enabled.\n" +
		"2) <code>nginx -t &amp;&amp; systemctl reload nginx</code>\n" +
		"3) TLS: <code>certbot --nginx -d " + htmlEscape(dom) + "</code>\n\n" +
		"<pre>" + htmlEscape(conf) + "</pre>\n" +
		"Опубликуй на хосте только порт " + port + " (он уже слушается контейнером)."
}
