package security

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func testKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("payload sensível de memória")
	env, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	out, err := enc.Decrypt(env)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(out) != string(plaintext) {
		t.Fatalf("round trip mismatch: %q", out)
	}
}

func TestDecryptDetectsTampering(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatal(err)
	}
	env, err := enc.Encrypt([]byte("data"))
	if err != nil {
		t.Fatal(err)
	}
	env.Ciphertext[0] ^= 0xFF
	if _, err := enc.Decrypt(env); err == nil {
		t.Fatal("expected tamper detection")
	}
}

func TestNewEncryptorRejectsBadKeys(t *testing.T) {
	if _, err := NewEncryptor("not-base64!!"); err == nil {
		t.Fatal("expected error for invalid base64")
	}
	short := base64.StdEncoding.EncodeToString([]byte("short"))
	if _, err := NewEncryptor(short); err == nil {
		t.Fatal("expected error for short key")
	}
}

type mapStore struct{ data map[string][]byte }

func (m *mapStore) Put(_ context.Context, key string, data []byte) error {
	m.data[key] = data
	return nil
}
func (m *mapStore) Get(_ context.Context, key string) ([]byte, error) {
	return m.data[key], nil
}
func (m *mapStore) Delete(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func TestEncryptingObjectStore(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatal(err)
	}
	inner := &mapStore{data: map[string][]byte{}}
	store := NewEncryptingObjectStore(inner, enc)

	ctx := context.Background()
	if err := store.Put(ctx, "k", []byte("secret")); err != nil {
		t.Fatalf("put: %v", err)
	}
	if string(inner.data["k"]) == "secret" {
		t.Fatal("inner store must hold ciphertext, not plaintext")
	}
	out, err := store.Get(ctx, "k")
	if err != nil || string(out) != "secret" {
		t.Fatalf("get: %q %v", out, err)
	}
}
