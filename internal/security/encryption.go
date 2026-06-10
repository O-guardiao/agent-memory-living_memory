package security

// Add envelope encryption for sensitive memory payloads here.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Envelope carries a payload encrypted with a per-object data key (DEK)
// that is itself wrapped by the key-encryption key (KEK). Fields are
// base64-encoded by encoding/json's []byte handling.
type Envelope struct {
	WrappedKey []byte `json:"wrapped_key"`
	KeyNonce   []byte `json:"key_nonce"`
	DataNonce  []byte `json:"data_nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

type Encryptor struct {
	kek cipher.AEAD
}

// NewEncryptor builds an AES-256-GCM envelope encryptor from a base64
// encoded 32-byte key.
func NewEncryptor(base64Key string) (*Encryptor, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes (base64-encoded)")
	}
	kek, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return &Encryptor{kek: kek}, nil
}

func (e *Encryptor) Encrypt(plaintext []byte) (Envelope, error) {
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return Envelope{}, err
	}
	dataAEAD, err := newAEAD(dek)
	if err != nil {
		return Envelope{}, err
	}
	dataNonce := make([]byte, dataAEAD.NonceSize())
	if _, err := io.ReadFull(rand.Reader, dataNonce); err != nil {
		return Envelope{}, err
	}
	keyNonce := make([]byte, e.kek.NonceSize())
	if _, err := io.ReadFull(rand.Reader, keyNonce); err != nil {
		return Envelope{}, err
	}
	return Envelope{
		WrappedKey: e.kek.Seal(nil, keyNonce, dek, nil),
		KeyNonce:   keyNonce,
		DataNonce:  dataNonce,
		Ciphertext: dataAEAD.Seal(nil, dataNonce, plaintext, nil),
	}, nil
}

func (e *Encryptor) Decrypt(env Envelope) ([]byte, error) {
	dek, err := e.kek.Open(nil, env.KeyNonce, env.WrappedKey, nil)
	if err != nil {
		return nil, fmt.Errorf("unwrap data key: %w", err)
	}
	dataAEAD, err := newAEAD(dek)
	if err != nil {
		return nil, err
	}
	plaintext, err := dataAEAD.Open(nil, env.DataNonce, env.Ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt payload: %w", err)
	}
	return plaintext, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
