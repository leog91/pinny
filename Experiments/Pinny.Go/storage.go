//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// The field names and types match the JSON used by the C# Pinny builds.
type noteState struct {
	ID       string  `json:"Id"`
	Text     string  `json:"Text"`
	Left     float64 `json:"Left"`
	Top      float64 `json:"Top"`
	Width    float64 `json:"Width"`
	Height   float64 `json:"Height"`
	IsPinned bool    `json:"IsPinned"`
	Theme    string  `json:"Theme"`
}

func newNoteID() (string, error) {
	var id struct {
		data1 uint32
		data2 uint16
		data3 uint16
		data4 [8]byte
	}
	result, _, callErr := syscall.NewLazyDLL("ole32.dll").NewProc("CoCreateGuid").Call(uintptr(unsafe.Pointer(&id)))
	if result != 0 {
		return "", fmt.Errorf("CoCreateGuid failed: %v", callErr)
	}
	return fmt.Sprintf("%08x-%04x-%04x-%02x%02x-%x", id.data1, id.data2, id.data3, id.data4[0], id.data4[1], id.data4[2:]), nil
}

func defaultDataDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", errors.New("LOCALAPPDATA is not set; pass --data-dir to choose a notes folder")
	}
	return filepath.Join(base, "Pinny.Go"), nil
}

func loadNotes(dir string) ([]noteState, error) {
	path := filepath.Join(dir, "notes.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		legacy, legacyErr := os.ReadFile(filepath.Join(dir, "note.json"))
		if errors.Is(legacyErr, os.ErrNotExist) {
			return nil, nil
		}
		if legacyErr != nil {
			return nil, legacyErr
		}
		var note noteState
		if err := json.Unmarshal(legacy, &note); err != nil {
			return nil, fmt.Errorf("read legacy note: %w", err)
		}
		return []noteState{note}, nil
	}
	if err != nil {
		return nil, err
	}
	var notes []noteState
	if err := json.Unmarshal(data, &notes); err != nil {
		return nil, fmt.Errorf("read notes: %w", err)
	}
	return notes, nil
}

func saveNotes(dir string, notes []noteState) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if notes == nil {
		notes = []noteState{}
	}
	data, err := json.MarshalIndent(notes, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(dir, "notes-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, filepath.Join(dir, "notes.json"))
}
