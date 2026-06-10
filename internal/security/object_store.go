package security

import (
	"context"
	"encoding/json"

	"github.com/agent-memory/agent-memory/internal/ports"
)

// EncryptingObjectStore wraps any ports.ObjectStore with envelope
// encryption: Put stores a JSON envelope, Get transparently decrypts.
type EncryptingObjectStore struct {
	inner ports.ObjectStore
	enc   *Encryptor
}

func NewEncryptingObjectStore(inner ports.ObjectStore, enc *Encryptor) *EncryptingObjectStore {
	return &EncryptingObjectStore{inner: inner, enc: enc}
}

func (s *EncryptingObjectStore) Put(ctx context.Context, key string, data []byte) error {
	env, err := s.enc.Encrypt(data)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return s.inner.Put(ctx, key, payload)
}

func (s *EncryptingObjectStore) Get(ctx context.Context, key string) ([]byte, error) {
	payload, err := s.inner.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var env Envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, err
	}
	return s.enc.Decrypt(env)
}

func (s *EncryptingObjectStore) Delete(ctx context.Context, key string) error {
	return s.inner.Delete(ctx, key)
}
