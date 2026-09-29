//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	stdstrings "strings"
	"unsafe"
)

type notesBackup struct {
	Version int         `json:"Version"`
	Notes   []noteState `json:"Notes"`
	Trash   []noteState `json:"Trash"`
}

func writeBackup(path string, notes, deleted []noteState) error {
	if notes == nil {
		notes = []noteState{}
	}
	if deleted == nil {
		deleted = []noteState{}
	}
	data, err := json.MarshalIndent(notesBackup{1, notes, deleted}, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(data, '\n'))
}

func readBackup(path string) (notesBackup, error) {
	var backup notesBackup
	data, err := os.ReadFile(path)
	if err != nil {
		return backup, err
	}
	data = bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
	var notesJSON, trashJSON json.RawMessage
	if len(data) > 0 && data[0] == '[' {
		backup.Version = 1
		notesJSON = data // Also accept the existing notes.json array format.
	} else {
		var envelope struct {
			Version      int
			Notes, Trash json.RawMessage
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return backup, fmt.Errorf("invalid JSON backup: %w", err)
		}
		if envelope.Version != 1 || len(envelope.Notes) == 0 || len(envelope.Trash) == 0 {
			return backup, errors.New("choose a Pinny backup or a notes.json array; this file is not a supported backup")
		}
		backup.Version, notesJSON, trashJSON = envelope.Version, envelope.Notes, envelope.Trash
	}
	backup.Notes, err = decodeImportedNotes(notesJSON)
	if err != nil {
		return backup, fmt.Errorf("invalid open notes: %w", err)
	}
	if len(trashJSON) > 0 {
		backup.Trash, err = decodeImportedNotes(trashJSON)
		if err != nil {
			return backup, fmt.Errorf("invalid Trash: %w", err)
		}
	}
	return backup, nil
}

func decodeImportedNotes(data []byte) ([]noteState, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '[' {
		return nil, errors.New("expected an array of notes")
	}
	var fields []map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for i, field := range fields {
		for _, key := range []string{"Text", "Left", "Top", "Width", "Height", "IsPinned", "Theme"} {
			value, ok := field[key]
			if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return nil, fmt.Errorf("note %d is missing %s", i+1, key)
			}
		}
	}
	var notes []noteState
	if err := json.Unmarshal(data, &notes); err != nil {
		return nil, err
	}
	for i, n := range notes {
		if stdstrings.ContainsRune(n.Text, 0) || stdstrings.ContainsRune(n.ID, 0) {
			return nil, fmt.Errorf("note %d contains a null character", i+1)
		}
		if n.Width < 0 || n.Height < 0 {
			return nil, fmt.Errorf("note %d has a negative size", i+1)
		}
	}
	return notes, nil
}

// Preserve IDs unless they collide with an existing or another imported note.
func prepareImport(backup notesBackup, existing, deleted []noteState) (notesBackup, error) {
	used := make(map[string]bool)
	for _, group := range [][]noteState{existing, deleted} {
		for _, n := range group {
			used[n.ID] = true
		}
	}
	backup.Notes = append([]noteState(nil), backup.Notes...)
	backup.Trash = append([]noteState(nil), backup.Trash...)
	for _, group := range [][]noteState{backup.Notes, backup.Trash} {
		for i := range group {
			if group[i].ID == "" || used[group[i].ID] {
				id, err := newNoteID()
				if err != nil {
					return backup, err
				}
				group[i].ID = id
			}
			used[group[i].ID] = true
		}
	}
	return backup, nil
}

func samePath(a, b string) bool {
	left, errA := filepath.Abs(a)
	right, errB := filepath.Abs(b)
	if errA == nil && errB == nil && stdstrings.EqualFold(left, right) {
		return true
	}
	leftInfo, errA := os.Stat(a)
	rightInfo, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(leftInfo, rightInfo)
}

func exportNotes(owner uintptr) {
	if !persistenceEnabled {
		showInfo(owner, "Export is unavailable in --notes mode.")
		return
	}
	path, err := choosePath(owner, "Export all notes and Trash", "", true, false)
	if err != nil {
		showOperationError(owner, "Export", err)
		return
	}
	if path == "" {
		return
	}
	backup, err := exportNotesToFile(path)
	if err != nil {
		showOperationError(owner, "Export", err)
		return
	}
	showInfo(owner, fmt.Sprintf("Exported %d open notes and %d trashed notes to:\n\n%s", len(backup.Notes), len(backup.Trash), path))
}

func exportNotesToFile(path string) (notesBackup, error) {
	config, _ := defaultDataDir()
	for _, protected := range []string{filepath.Join(dataDir, "notes.json"), filepath.Join(dataDir, "trash.json"), filepath.Join(dataDir, "note.json"), filepath.Join(config, "settings.json")} {
		if samePath(path, protected) {
			return notesBackup{}, errors.New("choose a separate backup file, rather than an active notes or settings file")
		}
	}
	notes, err := snapshotNotes(0)
	if err == nil {
		err = writeBackup(path, notes, trash)
	}
	if err != nil {
		return notesBackup{}, err
	}
	return notesBackup{1, notes, append([]noteState(nil), trash...)}, nil
}

func importNotes(owner uintptr) {
	if !persistenceEnabled {
		showInfo(owner, "Import is unavailable in --notes mode.")
		return
	}
	path, err := choosePath(owner, "Import notes (add to existing notes)", "", false, false)
	if err != nil {
		showOperationError(owner, "Import", err)
		return
	}
	if path == "" {
		return
	}
	backup, err := readBackup(path)
	if err != nil {
		showOperationError(owner, "Import", err)
		return
	}
	if len(backup.Notes) == 0 && len(backup.Trash) == 0 {
		showInfo(owner, "This backup contains no notes.")
		return
	}
	if err := addImportedNotes(backup); err != nil {
		showOperationError(owner, "Import", err)
		return
	}
	showInfo(owner, fmt.Sprintf("Added %d open notes and %d trashed notes. Your existing notes were kept.", len(backup.Notes), len(backup.Trash)))
}

func addImportedNotes(backup notesBackup) error {
	existing, err := snapshotNotes(0)
	if err != nil {
		return err
	}
	backup, err = prepareImport(backup, existing, trash)
	if err != nil {
		return err
	}
	stopSaveTimer()
	completed := false
	defer func() {
		if !completed && len(windowOrder) > 0 {
			queueSave(windowOrder[0])
		}
	}()
	start := len(windowOrder)
	rollbackWindows := func() {
		handles := append([]uintptr(nil), windowOrder[start:]...)
		for _, handle := range handles {
			call(destroyWindow, handle)
		}
	}
	for _, state := range backup.Notes {
		if err := showSavedNote(state); err != nil {
			rollbackWindows()
			return err
		}
	}
	updated := append(append([]noteState(nil), trash...), backup.Trash...)
	if err := saveTrash(dataDir, updated); err != nil {
		rollbackWindows()
		return err
	}
	if err := saveAllExcept(0); err != nil {
		rollbackWindows()
		if rollbackErr := saveTrash(dataDir, trash); rollbackErr != nil {
			trash = updated
			refreshTrashViewer()
			return fmt.Errorf("notes were not imported, but imported Trash was saved; %v; could not undo Trash: %w", err, rollbackErr)
		}
		return err
	}
	trash = updated
	refreshTrashViewer()
	completed = true
	return nil
}

func showOperationError(owner uintptr, operation string, err error) {
	call(messageBox, owner, uintptr(unsafe.Pointer(wide(operation+" could not be completed.\n\n"+err.Error()))),
		uintptr(unsafe.Pointer(wide("Pinny — "+operation))), 0x10)
}
