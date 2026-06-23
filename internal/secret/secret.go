// Package secret шифрует чувствительные значения (подписка чекера, SSH-ключи)
// перед записью в БД. NaCl secretbox (XSalsa20-Poly1305), ключ из SECRET_KEY
// (32 байта hex). Утечка дампа БД без ключа не раскрывает секреты (ПЛАН §13).
package secret

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"

	"golang.org/x/crypto/nacl/secretbox"
)

type Box struct {
	key [32]byte
}

// New принимает 32-байтный ключ в hex (64 hex-символа), напр. openssl rand -hex 32.
func New(hexKey string) (*Box, error) {
	raw, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, errors.New("SECRET_KEY: не hex")
	}
	if len(raw) != 32 {
		return nil, errors.New("SECRET_KEY: нужно ровно 32 байта (64 hex-символа)")
	}
	var b Box
	copy(b.key[:], raw)
	return &b, nil
}

// Encrypt возвращает nonce и ciphertext.
func (b *Box) Encrypt(plaintext []byte) (nonce, ciphertext []byte, err error) {
	var n [24]byte
	if _, err := rand.Read(n[:]); err != nil {
		return nil, nil, err
	}
	ct := secretbox.Seal(nil, plaintext, &n, &b.key)
	return n[:], ct, nil
}

// Decrypt расшифровывает по nonce и ciphertext.
func (b *Box) Decrypt(nonce, ciphertext []byte) ([]byte, error) {
	if len(nonce) != 24 {
		return nil, errors.New("неверный nonce")
	}
	var n [24]byte
	copy(n[:], nonce)
	pt, ok := secretbox.Open(nil, ciphertext, &n, &b.key)
	if !ok {
		return nil, errors.New("не удалось расшифровать (неверный ключ или повреждённые данные)")
	}
	return pt, nil
}

// EnsureKeyFile читает 32-байтный ключ (hex) из path или, если его нет/он
// невалиден, генерирует новый и сохраняет (0600). Позволяет работать без
// ручного SECRET_KEY — ключ живёт в томе данных и переиспользуется при рестарте.
func EnsureKeyFile(path string) (string, error) {
	if data, err := os.ReadFile(path); err == nil {
		k := strings.TrimSpace(string(data))
		if _, e := New(k); e == nil {
			return k, nil
		}
	}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	k := hex.EncodeToString(b[:])
	if err := os.WriteFile(path, []byte(k), 0o600); err != nil {
		return "", err
	}
	return k, nil
}
