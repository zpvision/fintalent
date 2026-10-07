package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

func resetOutboxCipher() (cipher.AEAD, error) {
	secret, err := passwordResetSecret()
	if err != nil {
		return nil, err
	}
	key := sha256.Sum256(append([]byte("fintalent:reset-outbox:v1:"), secret...))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encryptResetCode(code string) (string, error) {
	box, err := resetOutboxCipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, box.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	data := box.Seal(nonce, nonce, []byte(code), []byte("reset-code"))
	return "v1:" + base64.RawURLEncoding.EncodeToString(data), nil
}

func decryptResetCode(value string) (string, error) {
	if !strings.HasPrefix(value, "v1:") {
		return "", errors.New("invalid reset envelope")
	}
	box, err := resetOutboxCipher()
	if err != nil {
		return "", err
	}
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, "v1:"))
	if err != nil || len(data) < box.NonceSize()+box.Overhead() {
		return "", errors.New("invalid reset envelope")
	}
	plain, err := box.Open(nil, data[:box.NonceSize()], data[box.NonceSize():], []byte("reset-code"))
	if err != nil {
		return "", errors.New("invalid reset envelope")
	}
	return string(plain), nil
}
