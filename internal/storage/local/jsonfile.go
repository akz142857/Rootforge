// Package local provides single-process durable storage for local Rootforge deployments.
package local

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func ensureDataDirectory(directory string) (string, error) {
	if directory == "" {
		return "", errors.New("local storage directory is required")
	}
	cleaned := filepath.Clean(directory)
	if err := os.MkdirAll(cleaned, 0o700); err != nil {
		return "", fmt.Errorf("create local storage directory: %w", err)
	}
	return cleaned, nil
}

func readJSONFile(path string, target any) (bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return false, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return false, fmt.Errorf("decode %s: trailing data: %w", filepath.Base(path), err)
	}
	return true, nil
}

func writeJSONAtomic(path string, value any) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".rootforge-state-*")
	if err != nil {
		return fmt.Errorf("create temporary state file: %w", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set state file permissions: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("encode state file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync state file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close state file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace state file: %w", err)
	}
	removeTemporary = false

	directoryHandle, err := os.Open(directory)
	if err != nil {
		return nil
	}
	defer directoryHandle.Close()
	// The rename is already visible. Directory sync is best-effort because some
	// supported local filesystems reject it even though atomic rename succeeded.
	_ = directoryHandle.Sync()
	return nil
}
