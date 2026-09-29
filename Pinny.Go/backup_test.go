//go:build windows

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func sampleNote(id string) noteState {
	return noteState{ID: id, Text: "Hello 👋\r\nSecond line", Left: -100, Top: 200, Width: 400, Height: 300, IsPinned: true, Theme: "Paper"}
}

func TestBackupRoundTripAndLegacyImport(t *testing.T) {
	dir := t.TempDir()
	open, deleted := []noteState{sampleNote("open")}, []noteState{sampleNote("deleted")}
	path := filepath.Join(dir, "backup.json")
	if err := writeBackup(path, open, deleted); err != nil {
		t.Fatal(err)
	}
	got, err := readBackup(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || !reflect.DeepEqual(got.Notes, open) || !reflect.DeepEqual(got.Trash, deleted) {
		t.Fatalf("round trip lost note state: %+v", got)
	}
	if err := saveNotes(dir, open); err != nil {
		t.Fatal(err)
	}
	legacy, err := os.ReadFile(filepath.Join(dir, "notes.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append([]byte{0xef, 0xbb, 0xbf}, legacy...), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = readBackup(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Notes, open) || len(got.Trash) != 0 {
		t.Fatalf("legacy array import failed: %+v", got)
	}
}

func TestBackupRejectsInvalidFiles(t *testing.T) {
	for _, data := range []string{
		"", "not json", "{}", "null", "[null]", "[{}]", "[] trailing",
		`{"Version":2,"Notes":[],"Trash":[]}`,
		`{"Version":1,"Notes":[],"Trash":null}`,
		`{"Version":1,"Notes":[],"Trash":[{}]}`,
		`[{"Text":null,"Left":0,"Top":0,"Width":320,"Height":320,"IsPinned":false,"Theme":"Light"}]`,
		`[{"Text":123,"Left":0,"Top":0,"Width":320,"Height":320,"IsPinned":false,"Theme":"Light"}]`,
		`[{"Text":"bad\u0000text","Left":0,"Top":0,"Width":320,"Height":320,"IsPinned":false,"Theme":"Light"}]`,
	} {
		t.Run(data, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.json")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := readBackup(path); err == nil {
				t.Fatalf("accepted invalid backup: %s", data)
			}
		})
	}
	if _, err := decodeImportedNotes([]byte("   ")); err == nil {
		t.Fatal("accepted whitespace")
	}
}

func TestImportIDsDoNotCollideOrMutateSource(t *testing.T) {
	backup := notesBackup{Version: 1, Notes: []noteState{sampleNote("existing"), sampleNote("unique"), sampleNote("")}, Trash: []noteState{sampleNote("deleted"), sampleNote("unique")}}
	got, err := prepareImport(backup, []noteState{sampleNote("existing")}, []noteState{sampleNote("deleted")})
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{"existing": true, "deleted": true}
	for _, group := range [][]noteState{got.Notes, got.Trash} {
		for _, n := range group {
			if n.ID == "" || used[n.ID] {
				t.Fatalf("duplicate ID: %q", n.ID)
			}
			used[n.ID] = true
			if n.Text != sampleNote("").Text || n.Theme != "Paper" || !n.IsPinned {
				t.Fatal("import changed note contents")
			}
		}
	}
	if backup.Notes[0].ID != "existing" || backup.Trash[1].ID != "unique" {
		t.Fatal("mutated original backup")
	}
}

func TestFolderCopyAndRememberedLocation(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	defaultDir, err := defaultDataDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := savedDataDir()
	if err != nil || got != defaultDir {
		t.Fatalf("default location: %q, %v", got, err)
	}
	source, destination := t.TempDir(), t.TempDir()
	open, deleted := []noteState{sampleNote("open")}, []noteState{sampleNote("deleted")}
	if err := saveNotes(source, open); err != nil {
		t.Fatal(err)
	}
	if err := saveTrash(source, deleted); err != nil {
		t.Fatal(err)
	}
	if err := copyNotesToFolder(destination, open, deleted); err != nil {
		t.Fatal(err)
	}
	if err := saveDataDir(destination); err != nil {
		t.Fatal(err)
	}
	got, err = savedDataDir()
	if err != nil || got != destination {
		t.Fatalf("remembered location: %q, %v", got, err)
	}
	for _, dir := range []string{source, destination} {
		gotNotes, err := loadNotes(dir)
		if err != nil || !reflect.DeepEqual(gotNotes, open) {
			t.Fatalf("open notes changed in %s: %v", dir, err)
		}
		gotTrash, err := loadTrash(dir)
		if err != nil || !reflect.DeepEqual(gotTrash, deleted) {
			t.Fatalf("Trash changed in %s: %v", dir, err)
		}
	}
}

func TestFolderCopyDoesNotOverwriteExistingFiles(t *testing.T) {
	for _, name := range []string{"notes.json", "trash.json", "note.json"} {
		t.Run(name, func(t *testing.T) {
			destination := t.TempDir()
			path := filepath.Join(destination, name)
			original := []byte("existing notes must stay intact")
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			if err := copyNotesToFolder(destination, []noteState{sampleNote("new")}, nil); err == nil {
				t.Fatal("overwrote existing store")
			}
			got, err := os.ReadFile(path)
			if err != nil || !reflect.DeepEqual(got, original) {
				t.Fatalf("original file changed: %v", err)
			}
		})
	}
}

func TestSamePathRecognizesAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.json")
	if err := os.WriteFile(path, []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	if !samePath(path, filepath.Join(dir, "sub", "..", "notes.json")) {
		t.Fatal("did not recognize normalized path")
	}
	alias := filepath.Join(dir, "alias.json")
	if err := os.Link(path, alias); err != nil {
		t.Skipf("hardlinks unavailable: %v", err)
	}
	if !samePath(path, alias) {
		t.Fatal("did not recognize hardlink")
	}
}
