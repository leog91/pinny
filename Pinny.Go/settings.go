//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type appSettings struct {
	DataDir string `json:"DataDir"`
}

func savedDataDir() (string, error) {
	dir, err := defaultDataDir()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if errors.Is(err, os.ErrNotExist) {
		return dir, nil
	}
	if err != nil {
		return "", err
	}
	var settings appSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return "", fmt.Errorf("read notes folder setting: %w", err)
	}
	if settings.DataDir == "" {
		return dir, nil
	}
	return settings.DataDir, nil
}

func saveDataDir(dir string) error {
	settingsDir, err := defaultDataDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(settingsDir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(appSettings{dir}, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(settingsDir, "settings.json"), append(data, '\n'))
}

func copyNotesToFolder(destination string, notes, deleted []noteState) error {
	// Do not replace a different collection or a legacy single-note store.
	for _, name := range []string{"notes.json", "trash.json", "note.json"} {
		_, err := os.Lstat(filepath.Join(destination, name))
		if err == nil {
			return errors.New("this folder already contains Pinny notes; choose a folder without notes.json, trash.json, or note.json")
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := saveTrash(destination, deleted); err != nil {
		return err
	}
	if err := saveNotes(destination, notes); err != nil {
		return fmt.Errorf("could not copy notes; the original folder is still active: %w", err)
	}
	return nil
}

func changeNotesFolder(owner uintptr) {
	if !persistenceEnabled {
		showInfo(owner, "Choosing a notes folder is unavailable in --notes mode.")
		return
	}
	destination, err := choosePath(owner, "Choose a folder for your notes", dataDir, false, true)
	if err != nil {
		showOperationError(owner, "Change notes folder", err)
		return
	}
	if destination == "" {
		return
	}
	if samePath(destination, dataDir) {
		showInfo(owner, "Your notes already use this folder:\n\n"+dataDir)
		return
	}
	if err := switchNotesFolder(destination); err != nil {
		showOperationError(owner, "Change notes folder", err)
		return
	}
	message := "Your notes and Trash now save to:\n\n" + dataDir + "\n\nThe original files were kept. This folder is remembered for future launches."
	if dataDirOverridden {
		message += "\n\nA --data-dir option or PINNY_DATA_DIR setting still takes precedence when launching Pinny."
	}
	showInfo(owner, message)
}

func switchNotesFolder(destination string) error {
	destination, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	if samePath(destination, dataDir) {
		return nil
	}
	// Save pending edits in the source folder before making its copy.
	stopSaveTimer()
	if err := saveAllExcept(0); err != nil {
		return err
	}
	notes, err := snapshotNotes(0)
	if err != nil {
		return err
	}
	if err := copyNotesToFolder(destination, notes, trash); err != nil {
		return err
	}
	if err := saveDataDir(destination); err != nil {
		return fmt.Errorf("notes were copied, but the folder could not be remembered; the original folder is still active: %w", err)
	}
	dataDir = destination
	return nil
}
