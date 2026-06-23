package store

import "errors"

// ErrSecretsDisabled возвращается, когда не задан SECRET_KEY.
var ErrSecretsDisabled = errors.New("секреты выключены: задай SECRET_KEY (openssl rand -hex 32)")

// SetSecret шифрует и сохраняет значение под ключом k.
func (s *Store) SetSecret(k, plaintext string) error {
	if s.box == nil {
		return ErrSecretsDisabled
	}
	nonce, ct, err := s.box.Encrypt([]byte(plaintext))
	if err != nil {
		return err
	}
	_, err = s.exec(`INSERT INTO secrets(k,nonce,ciphertext) VALUES(?,?,?)
		ON CONFLICT(k) DO UPDATE SET nonce=excluded.nonce, ciphertext=excluded.ciphertext`,
		k, nonce, ct)
	return err
}

// GetSecret расшифровывает значение по ключу k.
func (s *Store) GetSecret(k string) (string, error) {
	if s.box == nil {
		return "", ErrSecretsDisabled
	}
	var nonce, ct []byte
	if err := s.queryRow(`SELECT nonce,ciphertext FROM secrets WHERE k=?`, k).Scan(&nonce, &ct); err != nil {
		return "", err
	}
	pt, err := s.box.Decrypt(nonce, ct)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// HasSecret — есть ли сохранённый секрет под ключом k.
func (s *Store) HasSecret(k string) bool {
	var one int
	return s.queryRow(`SELECT 1 FROM secrets WHERE k=?`, k).Scan(&one) == nil
}
