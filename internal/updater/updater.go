// Package updater реализует самообновление одиночного бинаря: скачивает новую
// сборку (из ветки go-build, по UPDATE_URL), проверяет, атомарно заменяет
// текущий исполняемый файл и ре-эксекает процесс.
//
// Подходит для bare-metal/бинарных деплоев. В Docker правильнее обновлять образ
// (docker compose pull && up -d) — там подмена бинаря внутри контейнера
// потеряется при пересоздании; об этом сообщает бот, если UPDATE_URL не задан.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type Updater struct {
	url        string
	shaURL     string
	commitsAPI string
	client     *http.Client
}

func New(url, shaURL string) *Updater {
	return &Updater{
		url:        url,
		shaURL:     shaURL,
		commitsAPI: "https://api.github.com/repos/Mrvibecodic/xray-checker-statuspage/commits/go-build",
		client:     &http.Client{Timeout: 120 * time.Second},
	}
}

// manualHint — фолбэк-подсказка для ручного обновления (Docker).
const manualHint = "Обнови вручную: docker compose pull && docker compose up -d"

// CheckResult — итог проверки обновления.
type CheckResult struct {
	HasUpdate bool
	Latest    string // короткий sha последнего коммита go-build
	Message   string // первая строка сообщения коммита
}

// Check спрашивает у GitHub API последний коммит ветки go-build и сравнивает с
// текущей версией (в неё зашит короткий sha сборки).
func (u *Updater) Check(ctx context.Context, currentVersion string) (CheckResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.commitsAPI, nil)
	if err != nil {
		return CheckResult{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "xray-status-updater")
	resp, err := u.client.Do(req)
	if err != nil {
		return CheckResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return CheckResult{}, fmt.Errorf("GitHub ответил %d. %s", resp.StatusCode, manualHint)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return CheckResult{}, fmt.Errorf("не удалось прочитать ответ GitHub. %s", manualHint)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return CheckResult{}, fmt.Errorf("пустой ответ от GitHub. %s", manualHint)
	}
	var v struct {
		Sha    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return CheckResult{}, fmt.Errorf("не удалось разобрать ответ GitHub. %s", manualHint)
	}
	short := v.Sha
	if len(short) > 7 {
		short = short[:7]
	}
	msg := v.Commit.Message
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = msg[:i]
	}
	return CheckResult{
		HasUpdate: short != "" && !strings.Contains(currentVersion, short),
		Latest:    short,
		Message:   msg,
	}, nil
}

// Available — настроено ли самообновление (задан URL).
func (u *Updater) Available() bool { return strings.TrimSpace(u.url) != "" }

// Fetch скачивает новый бинарь в память.
func (u *Updater) Fetch(ctx context.Context) ([]byte, error) {
	if err := requireHTTPS(u.url); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update download: status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 200<<20))
}

// ValidateELF проверяет, что данные — Linux ELF-бинарь.
func ValidateELF(b []byte) error {
	if len(b) < 4 || b[0] != 0x7f || b[1] != 'E' || b[2] != 'L' || b[3] != 'F' {
		return fmt.Errorf("скачанный файл не похож на ELF-бинарь")
	}
	return nil
}

// verifySHA сверяет sha256(data) с содержимым shaURL (hex, опционально с именем файла).
func (u *Updater) verifySHA(ctx context.Context, data []byte) error {
	if strings.TrimSpace(u.shaURL) == "" {
		return nil
	}
	if err := requireHTTPS(u.shaURL); err != nil {
		return err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.shaURL, nil)
	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	fields := strings.Fields(strings.TrimSpace(string(raw)))
	if len(fields) == 0 {
		return fmt.Errorf("sha256-файл пуст или не читается")
	}
	want := strings.ToLower(fields[0])
	if len(want) != 64 {
		return fmt.Errorf("sha256-файл повреждён (неверная длина хеша)")
	}
	got := hex.EncodeToString(sha256Sum(data))
	if want != got {
		return fmt.Errorf("sha256 не совпал: ожидался %s, получен %s", want, got)
	}
	return nil
}

// requireHTTPS не даёт качать обновление по http/file/прочим схемам — только
// https, чтобы канал доставки бинаря нельзя было подменить downgrade'ом.
func requireHTTPS(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("некорректный URL обновления: %w", err)
	}
	// https обязателен в проде; http допускаем только на loopback (локальные
	// тесты/прокси) — там downgrade-подмены по сети нет.
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && isLoopbackHost(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("URL обновления должен быть https, а не %q", u.Scheme)
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func sha256Sum(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

// ReplaceExecutable атомарно заменяет файл target новыми данными (temp+rename в
// той же директории; старый сохраняется как target.old и удаляется при успехе).
func ReplaceExecutable(target string, data []byte) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".upd-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // если останется при ошибке
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}
	backup := target + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(target, backup); err != nil {
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Rename(backup, target) // откат
		return err
	}
	_ = os.Remove(backup)
	return nil
}

// Apply скачивает, проверяет и заменяет текущий исполняемый файл. Возвращает путь
// к нему (для последующего ре-эксека).
func (u *Updater) Apply(ctx context.Context) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, _ = filepath.EvalSymlinks(exe)
	data, err := u.Fetch(ctx)
	if err != nil {
		return "", err
	}
	if err := ValidateELF(data); err != nil {
		return "", err
	}
	if err := u.verifySHA(ctx, data); err != nil {
		return "", err
	}
	if err := ReplaceExecutable(exe, data); err != nil {
		return "", err
	}
	return exe, nil
}

// Reexec заменяет текущий процесс новым бинарём (тем же argv/env).
func Reexec(path string) error {
	return syscall.Exec(path, os.Args, os.Environ())
}
