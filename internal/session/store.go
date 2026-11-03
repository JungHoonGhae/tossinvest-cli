package session

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/atomicfile"
)

var ErrNoSession = errors.New("no stored session")

type Session struct {
	Provider        string            `json:"provider"`
	Cookies         map[string]string `json:"cookies,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	Storage         map[string]string `json:"storage,omitempty"`
	RetrievedAt     time.Time         `json:"retrieved_at"`
	ExpiresAt       *time.Time        `json:"expires_at,omitempty"`
	ServerExpiresAt *time.Time        `json:"server_expires_at,omitempty"`
}

type Store interface {
	Load(context.Context) (*Session, error)
	Save(context.Context, *Session) error
	Clear(context.Context) error
}

type FileStore struct {
	path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (s Session) IsExpired(now time.Time) bool {
	if s.ExpiresAt == nil {
		return false
	}

	return now.After(*s.ExpiresAt)
}

func (s *FileStore) Load(context.Context) (*Session, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoSession
		}

		return nil, err
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}

	return &sess, nil
}

func (s *FileStore) Save(_ context.Context, sess *Session) error {
	return WriteFile(s.path, sess)
}

func (s *FileStore) Clear(_ context.Context) error {
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

func (s *FileStore) Path() string {
	return s.path
}

func WriteFile(path string, sess *Session) error {
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}

	return atomicfile.WriteFile(path, data, 0o600)
}
