package bot

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleFaviconUpload принимает картинку (документ/фото), скачивает её через
// Telegram и сохраняет как фавикон публичной страницы.
func (tb *Bot) handleFaviconUpload(ctx context.Context, chatID int64, msg *models.Message) {
	awaitMsg := tb.st.GetBotState(chatID, "await_msg")

	var fileID, mime string
	switch {
	case msg.Document != nil:
		fileID = msg.Document.FileID
		mime = msg.Document.MimeType
	case len(msg.Photo) > 0:
		fileID = msg.Photo[len(msg.Photo)-1].FileID
		mime = "image/jpeg"
	}
	if fileID == "" {
		tb.reply(ctx, chatID, "Пришли картинку фавикона — PNG/SVG/ICO (лучше как документ, чтобы без сжатия).")
		return
	}

	tb.st.DelBotState(chatID, "await")
	tb.st.DelBotState(chatID, "await_msg")
	_, _ = tb.b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: chatID, MessageID: msg.ID})

	note := "✅ Фавикон обновлён.\n\n"
	if f, err := tb.b.GetFile(ctx, &bot.GetFileParams{FileID: fileID}); err != nil {
		note = "❌ Не удалось получить файл: " + htmlEscape(err.Error()) + "\n\n"
	} else if data, derr := downloadLimited(ctx, tb.b.FileDownloadLink(f), 1<<20); derr != nil {
		note = "❌ Ошибка загрузки: " + htmlEscape(derr.Error()) + "\n\n"
	} else {
		if mime == "" {
			mime = sniffMime(f.FilePath, data)
		}
		if err := tb.st.SetAsset("favicon", mime, data); err != nil {
			note = "❌ Ошибка сохранения: " + htmlEscape(err.Error()) + "\n\n"
		} else {
			_ = tb.st.AddAudit(chatID, "favicon_set", "", mime, "ok")
		}
	}

	if mid, err := strconv.Atoi(awaitMsg); err == nil && mid > 0 {
		tb.editMessage(ctx, chatID, mid, note+tb.pageText(), tb.pageKB())
	} else {
		tb.sendSection(ctx, chatID, note+tb.pageText(), tb.pageKB())
	}
}

func downloadLimited(ctx context.Context, url string, max int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, max))
}

func sniffMime(path string, data []byte) string {
	p := strings.ToLower(path)
	switch {
	case strings.HasSuffix(p, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(p, ".png"):
		return "image/png"
	case strings.HasSuffix(p, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(p, ".jpg"), strings.HasSuffix(p, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(p, ".webp"):
		return "image/webp"
	}
	return http.DetectContentType(data)
}
