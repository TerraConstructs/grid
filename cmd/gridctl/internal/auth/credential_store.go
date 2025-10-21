package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/terraconstructs/grid/pkg/sdk"
)

// CredentialStore is an interface for storing and retrieving credentials.
type CredentialStore interface {
	SaveCredentials(credentials *sdk.Credentials) error
	LoadCredentials() (*sdk.Credentials, error)
	DeleteCredentials() error
}

const credentialsFile = "credentials.json"

// FileStore implements the CredentialStore interface using a JSON file.
type FileStore struct {
	path string
}

// NewFileStore creates a new FileStore.
func NewFileStore() (CredentialStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}
	gridDir := filepath.Join(home, ".grid")
	if err := os.MkdirAll(gridDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create .grid directory: %w", err)
	}
	return &FileStore{
		path: filepath.Join(gridDir, credentialsFile),
	}, nil
}

// SaveCredentials saves the credentials to the file.
func (s *FileStore) SaveCredentials(credentials *sdk.Credentials) error {
	data, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}
	return os.WriteFile(s.path, data, 0600)
}

// LoadCredentials loads the credentials from the file.
func (s *FileStore) LoadCredentials() (*sdk.Credentials, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not logged in")
		}
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}
	var creds sdk.Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}
	return &creds, nil
}

// DeleteCredentials deletes the credentials file.
func (s *FileStore) DeleteCredentials() error {
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(s.path)
}
