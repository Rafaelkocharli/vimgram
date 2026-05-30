// Package storage provides persistent storage adapters used by the
// telegram client (session cache, etc).
package storage

import "github.com/gotd/td/session"

// NewFileSession returns a SessionStorage backed by a file at the given path.
// It satisfies gotd's session.Storage interface.
func NewFileSession(path string) *session.FileStorage {
	return &session.FileStorage{Path: path}
}
