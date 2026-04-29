package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

type providerConfigCodec struct {
	aead cipher.AEAD
}

func newProviderConfigCodec(secret string) (*providerConfigCodec, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("provider config encryption key must be set")
	}

	sum := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, fmt.Errorf("create provider config cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create provider config AEAD: %w", err)
	}

	return &providerConfigCodec{aead: aead}, nil
}

func (c *providerConfigCodec) EncryptString(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate provider config nonce: %w", err)
	}

	ciphertext := c.aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, ciphertext...)
	return base64.RawStdEncoding.EncodeToString(payload), nil
}

func (c *providerConfigCodec) DecryptString(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	payload, err := base64.RawStdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode encrypted provider config: %w", err)
	}
	if len(payload) < c.aead.NonceSize() {
		return "", fmt.Errorf("decrypt provider config: malformed ciphertext")
	}

	nonce := payload[:c.aead.NonceSize()]
	sealed := payload[c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt provider config: %w", err)
	}

	return string(plaintext), nil
}
