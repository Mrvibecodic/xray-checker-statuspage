package webctl

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// selfSignedCert генерирует self-signed TLS-сертификат для домена. За Cloudflare
// в режиме SSL=Full этого достаточно: CF принимает любой серт на origin и
// рукопожатие завершается (никаких 525). Для режима Full(strict) нужен реальный
// серт — Cloudflare Origin Certificate через CERT_FILE/KEY_FILE.
func selfSignedCert(domain string) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: domain},
		DNSNames:              []string{domain},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
}

// loadOrCreateSelfSigned возвращает self-signed серт для домена, переиспользуя
// сохранённый в dir (PEM cert+key), пока он валиден и выписан на тот же домен.
// Иначе генерирует новый и сохраняет. Это убирает пересоздание серта на каждый
// старт/перезапуск веб-сервера и переживает рестарт процесса.
func loadOrCreateSelfSigned(dir, domain string) (tls.Certificate, error) {
	path := filepath.Join(dir, "self-signed.pem")
	if cert, err := loadSelfSigned(path, domain); err == nil {
		return cert, nil
	}
	cert, err := selfSignedCert(domain)
	if err != nil {
		return tls.Certificate{}, err
	}
	if leaf, perr := x509.ParseCertificate(cert.Certificate[0]); perr == nil {
		cert.Leaf = leaf
	}
	// Сбой записи на диск не критичен: серт валиден в памяти, просто не
	// закэшировался (на следующем старте сгенерируем заново).
	_ = saveSelfSigned(path, cert)
	return cert, nil
}

// loadSelfSigned читает закэшированный серт и принимает его, только если он ещё
// действителен (минимум сутки в запасе) и выписан на нужный домен.
func loadSelfSigned(path, domain string) (tls.Certificate, error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return tls.Certificate{}, err
	}
	cert, err := tls.X509KeyPair(pemBytes, pemBytes)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return tls.Certificate{}, err
	}
	if time.Now().Add(24 * time.Hour).After(leaf.NotAfter) {
		return tls.Certificate{}, errors.New("закэшированный self-signed серт скоро истекает")
	}
	if leaf.VerifyHostname(domain) != nil {
		return tls.Certificate{}, errors.New("закэшированный self-signed серт выписан на другой домен")
	}
	cert.Leaf = leaf
	return cert, nil
}

// saveSelfSigned пишет cert+key одним PEM-файлом (0600) в dir (0700).
func saveSelfSigned(path string, cert tls.Certificate) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	key, ok := cert.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		return errors.New("неподдерживаемый тип приватного ключа")
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}); err != nil {
		return err
	}
	if err := pem.Encode(&buf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o600)
}
