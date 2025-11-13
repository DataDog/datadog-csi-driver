// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var ErrItemNotFound = errors.New("item not found in store")

// Store provides a file based storage solution for packages. It is not thread safe and it is up to the caller to
// manage concurrency.
type Store struct {
	basePath string
}

// NewStore creates a new store and ensures the base path exists.
func NewStore(basePath string) (*Store, error) {
	// Create the base path if it doesn't exist.
	exists, err := directoryExistsAndNotEmpty(basePath)
	if err != nil {
		return nil, fmt.Errorf("could not determine if the bath path %s is valid: %w", basePath, err)
	}
	if !exists {
		err := os.MkdirAll(basePath, 0o755)
		if err != nil {
			return nil, fmt.Errorf("could not create base path %s: %w", basePath, err)
		}
	}

	// Return the store.
	return &Store{
		basePath: basePath,
	}, nil
}

// Add will move a source directory into the store. This is intended to be used with a downloader and scratch space. If
// a package already exists at the provided ID, it will not be re-added.
func (s *Store) Add(id string, src string) (string, error) {
	// Validate the id.
	if id == "" {
		return "", fmt.Errorf("id cannot be empty")
	}

	// If the package already exists, there is no more work to do.
	exists, err := s.exists(id)
	if err != nil {
		return "", err
	}
	if exists {
		return s.getPath(id), nil
	}

	// Ensure the source path is valid.
	exists, err = directoryExistsAndNotEmpty(src)
	if err != nil {
		return "", fmt.Errorf("could not determine if the source path %s is valid: %w", src, err)
	}
	if !exists {
		return "", fmt.Errorf("the source path %s must exist and be a non-empty directory", src)
	}

	// Move the source path into the store.
	dst := s.getPath(id)
	err = os.Rename(src, dst)
	if err != nil {
		return "", fmt.Errorf("could not add package with id %s to store: %w", id, err)
	}
	return dst, nil
}

// Get returns an item in the store if it exists.
func (s *Store) Get(id string) (string, error) {
	// Validate the id.
	if id == "" {
		return "", fmt.Errorf("id cannot be empty")
	}

	// Determine if item exists.
	exists, err := s.exists(id)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", ErrItemNotFound
	}

	// Return the item if it does.
	return s.getPath(id), nil
}

// Remove deletes an item from the store.
func (s *Store) Remove(id string) error {
	// Validate the id.
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}

	// If the item doesn't exist, there is no more work to be done.
	exists, err := s.exists(id)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	// Remove the item from disk.
	path := s.getPath(id)
	err = os.RemoveAll(path)
	if err != nil {
		return fmt.Errorf("could not remove package with id %s: %w", id, err)
	}

	return nil
}

// Exists determines if an item exists in the store.
func (s *Store) Exists(id string) (bool, error) {
	// Validate the id.
	if id == "" {
		return false, fmt.Errorf("id cannot be empty")
	}

	return s.exists(id)
}

func (s *Store) exists(id string) (bool, error) {
	// Determine if path eixsts.
	path := s.getPath(id)
	exists, err := directoryExistsAndNotEmpty(path)
	if err != nil {
		return false, fmt.Errorf("could not determine if package with id %s exists: %w", id, err)
	}
	return exists, nil
}

func (s *Store) getPath(id string) string {
	return filepath.Join(s.basePath, id)
}

func directoryExistsAndNotEmpty(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("could not determine if directory %s exists: %w", path, err)
	}

	if !info.IsDir() {
		return false, fmt.Errorf("the path %s is not a directory", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return false, fmt.Errorf("could not read directory %s: %w", path, err)
	}

	return len(entries) > 0, nil
}
