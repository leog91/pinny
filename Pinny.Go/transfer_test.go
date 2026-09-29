//go:build windows

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

// Exercise the real Win32 note windows and the same transfer functions as the menu.
func TestTransfersWithLiveNotes(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	t.Setenv("LOCALAPPDATA", t.TempDir())
	dataDir = t.TempDir()
	persistenceEnabled = true
	wc := wndClassEx{size: uint32(unsafe.Sizeof(wndClassEx{})), wndProc: syscall.NewCallback(wndProc), instance: call(getModuleHandle, 0), className: wide(windowClass)}
	if call(registerClass, uintptr(unsafe.Pointer(&wc))) == 0 {
		t.Fatal("could not register test note class")
	}
	for _, theme := range []string{"Light", "Dark", "Paper"} {
		palettes[theme] = makePalette(255, 255, 255, 244, 245, 247, 200, 205, 211, 225, 229, 235, 32, 35, 40, 85, 91, 99)
	}
	defer func() {
		stopSaveTimer()
		quitting = true
		for _, hwnd := range append([]uintptr(nil), windowOrder...) {
			call(destroyWindow, hwnd)
		}
		for _, p := range palettes {
			call(deleteObject, p.surface)
			call(deleteObject, p.header)
			call(deleteObject, p.border)
			call(deleteObject, p.hover)
		}
	}()
	for _, id := range []string{"first", "second"} {
		if err := showSavedNote(sampleNote(id)); err != nil {
			t.Fatal(err)
		}
	}
	trash = []noteState{sampleNote("deleted")}
	if err := saveAllExcept(0); err != nil {
		t.Fatal(err)
	}
	if err := saveTrash(dataDir, trash); err != nil {
		t.Fatal(err)
	}
	first := windowOrder[0]
	// Export must read live text, rather than a possibly stale notes.json.
	call(setWindowText, windows[first].edit, uintptr(unsafe.Pointer(wide("Unsaved export edit"))))
	path := filepath.Join(t.TempDir(), "backup.json")
	exported, err := exportNotesToFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(exported.Notes) != 2 || exported.Notes[0].Text != "Unsaved export edit" || len(exported.Trash) != 1 {
		t.Fatalf("export missed live state: %+v", exported)
	}
	if _, err := exportNotesToFile(filepath.Join(dataDir, "notes.json")); err == nil {
		t.Fatal("export accepted an active data file")
	}
	backup, err := readBackup(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := addImportedNotes(backup); err != nil {
		t.Fatal(err)
	}
	if len(windows) != 4 || len(trash) != 2 {
		t.Fatalf("import did not append the whole collection: %d windows, %d Trash", len(windows), len(trash))
	}
	saved, err := loadNotes(dataDir)
	if err != nil || len(saved) != 4 {
		t.Fatalf("import was not persisted: %v", err)
	}
	used := make(map[string]bool)
	for _, group := range [][]noteState{saved, trash} {
		for _, n := range group {
			if used[n.ID] {
				t.Fatalf("duplicate note ID after import: %s", n.ID)
			}
			used[n.ID] = true
		}
	}
	// Lock notes.json against replacement to check the partial-save recovery path.
	notesPath := filepath.Join(dataDir, "notes.json")
	lock, err := syscall.CreateFile(syscall.StringToUTF16Ptr(notesPath), syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	importErr := addImportedNotes(backup)
	syscall.CloseHandle(lock)
	stopSaveTimer()
	if importErr == nil {
		t.Fatal("import succeeded despite locked notes file")
	}
	if len(windows) != 4 || len(trash) != 2 {
		t.Fatal("failed import did not restore live state")
	}
	stillSaved, err := loadNotes(dataDir)
	if err != nil || !reflect.DeepEqual(saved, stillSaved) {
		t.Fatalf("failed import changed original notes: %v", err)
	}
	stillDeleted, err := loadTrash(dataDir)
	if err != nil || !reflect.DeepEqual(trash, stillDeleted) {
		t.Fatalf("failed import did not undo the Trash write: %v", err)
	}

	source := dataDir
	destination := t.TempDir()
	if err := switchNotesFolder(destination); err != nil {
		t.Fatal(err)
	}
	if dataDir != destination {
		t.Fatal("active folder did not change")
	}
	remembered, err := savedDataDir()
	if err != nil || remembered != destination {
		t.Fatalf("new folder was not remembered: %v", err)
	}
	if sourceNotes, err := loadNotes(source); err != nil || !reflect.DeepEqual(saved, sourceNotes) {
		t.Fatal("folder change changed original notes")
	}
	call(setWindowText, windows[first].edit, uintptr(unsafe.Pointer(wide("Edit in new folder"))))
	if err := saveAllExcept(0); err != nil {
		t.Fatal(err)
	}
	newNotes, err := loadNotes(destination)
	if err != nil || len(newNotes) != 4 || newNotes[0].Text != "Edit in new folder" {
		t.Fatalf("edits did not save to the new folder: %v", err)
	}
	if sourceNotes, err := loadNotes(source); err != nil || !reflect.DeepEqual(saved, sourceNotes) {
		t.Fatal("new edits changed the original folder")
	}
	newTrash, err := loadTrash(destination)
	if err != nil || !reflect.DeepEqual(trash, newTrash) {
		t.Fatalf("folder change lost Trash: %v", err)
	}

	// A failed settings write must keep the current active folder.
	settingsDir, _ := defaultDataDir()
	settingsPath := filepath.Join(settingsDir, "settings.json")
	settingsLock, err := syscall.CreateFile(syscall.StringToUTF16Ptr(settingsPath), syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	err = switchNotesFolder(t.TempDir())
	syscall.CloseHandle(settingsLock)
	if err == nil || dataDir != destination {
		t.Fatal("failed setting save changed the active folder")
	}
	remembered, err = savedDataDir()
	if err != nil || remembered != destination {
		t.Fatal("failed setting save changed the remembered folder")
	}
	if _, err := os.Stat(notesPath); err != nil {
		t.Fatal("original notes file is missing")
	}
}
