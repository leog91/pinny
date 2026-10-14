//go:build windows

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"pinny-go/internal/capture"
	"runtime"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// Real Win32 windows, real HTTP, and the UI-thread channel dispatcher. This test
// can run on Windows CI without manual interaction or the user's notes folder.
func TestSyncWithLiveNotes(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	t.Setenv("LOCALAPPDATA", t.TempDir())
	oldDir, oldPersistence, oldQuitting := dataDir, persistenceEnabled, quitting
	oldWindows, oldOrder, oldPalettes, oldTrash := windows, windowOrder, palettes, trash
	dataDir, persistenceEnabled, quitting = t.TempDir(), true, false
	windows, windowOrder, palettes, trash = map[uintptr]*note{}, nil, map[string]palette{}, nil
	pendingSyncReceipts = nil
	automaticSync = true
	wc := wndClassEx{size: uint32(unsafe.Sizeof(wndClassEx{})), wndProc: syscall.NewCallback(wndProc), instance: call(getModuleHandle, 0), className: wide(windowClass)}
	if call(registerClass, uintptr(unsafe.Pointer(&wc))) == 0 {
		t.Fatal("could not register test note class")
	}
	palettes["Light"] = makePalette(255, 255, 255, 244, 245, 247, 200, 205, 211, 225, 229, 235, 32, 35, 40, 85, 91, 99)
	defer func() {
		stopSync()
		stopSaveTimer()
		quitting = true
		for _, hwnd := range append([]uintptr(nil), windowOrder...) {
			call(destroyWindow, hwnd)
		}
		for _, p := range palettes {
			for _, brush := range []uintptr{p.surface, p.header, p.border, p.hover} {
				call(deleteObject, brush)
			}
		}
		call(user32.NewProc("UnregisterClassW"), uintptr(unsafe.Pointer(wide(windowClass))), wc.instance)
		dataDir, persistenceEnabled, quitting = oldDir, oldPersistence, oldQuitting
		windows, windowOrder, palettes, trash = oldWindows, oldOrder, oldPalettes, oldTrash
		syncBusy, syncTimer, syncPollTimer = false, 0, 0
		pendingSyncReceipts = nil
		syncConfig = capture.Config{}
	}()
	if err := showNote(initialNote{text: "Local note", width: 320, height: 320, x: 80, y: 80, theme: "Light"}); err != nil {
		t.Fatal(err)
	}
	if err := saveAllExcept(0); err != nil {
		t.Fatal(err)
	}
	localWindow := windowOrder[0]
	item := capture.Item{ID: "12345678-1234-4234-8234-123456789abc", Kind: "todo", Text: "Incoming test", Title: "Capture"}
	var gets, acks atomic.Int32
	var delivered atomic.Bool
	started, release := make(chan struct{}, 1), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/pending" {
			if gets.Add(1) == 1 {
				started <- struct{}{}
				<-release
			}
			items := []capture.Item{}
			if !delivered.Load() {
				items = append(items, item)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
			return
		}
		if r.URL.Path == "/api/v1/ack" {
			// Verify that both local files were persisted before any acknowledgment.
			saved, err := loadNotes(dataDir)
			if err != nil || len(saved) != 2 {
				t.Error("ack before notes persisted")
			}
			receipts, err := readReceipts("sync-receipts.json")
			if err != nil || !receipts[capture.NoteID(serverOrigin(r), item.ID)] {
				t.Error("ack before receipts persisted")
			}
			acks.Add(1)
			delivered.Store(true)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	syncConfig = capture.Config{Endpoint: server.URL, Credential: "synthetic-desktop-credential-32-characters"}
	startAutomaticSync()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("startup sync did not start")
	}
	// With the HTTP response still blocked, edit a real native control immediately.
	call(setWindowText, windows[localWindow].edit, uintptr(unsafe.Pointer(wide("Edited during request"))))
	startSync(false)
	if !syncBusy || gets.Load() != 1 {
		t.Fatal("overlapping sync or missing busy guard")
	}
	// Reject replacement of notes.json to prove no ack follows a failed local save.
	lock, err := syscall.CreateFile(syscall.StringToUTF16Ptr(filepath.Join(dataDir, "notes.json")), syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	pump := func() {
		deadline := time.Now().Add(5 * time.Second)
		for syncBusy && time.Now().Before(deadline) {
			handleSyncMessage(msg{message: wmTimer, wParam: syncPollTimer})
			time.Sleep(5 * time.Millisecond)
		}
		if syncBusy {
			t.Fatal("sync did not complete")
		}
	}
	pump()
	syscall.CloseHandle(lock)
	if acks.Load() != 0 || len(windows) != 2 {
		t.Fatal("failed save acknowledged or dropped incoming window")
	}
	toggleAutomaticSync(0)
	if automaticSync || syncTimer != 0 {
		t.Fatal("disable did not stop scheduled sync")
	}
	initializeSync()
	// No sync.json exists: configuration is retained here, preference reloads false.
	if automaticSync {
		t.Fatal("automatic preference did not persist")
	}
	before := gets.Load()
	startAutomaticSync()
	if syncBusy || gets.Load() != before {
		t.Fatal("disabled startup sync made a request")
	}
	startSync(false) // Same path as manual action, but no modal status box in this test.
	pump()
	if acks.Load() != 1 || len(windows) != 2 {
		t.Fatal("retry duplicated delivery or failed to acknowledge")
	}
	saved, err := loadNotes(dataDir)
	if err != nil || saved[0].Text != "Edited during request" || saved[1].Text != "[ ] Incoming test" {
		t.Fatal("incoming batch overwrote edits or todo text")
	}
	// Lost cloud ack / restart / permanent local deletion cannot recreate a receipt.
	call(destroyWindow, windowOrder[1])
	if err := saveAllExcept(0); err != nil {
		t.Fatal(err)
	}
	if err := applyIncoming(server.URL, []capture.Item{item}); err != nil {
		t.Fatal(err)
	}
	if len(windows) != 1 {
		t.Fatal("receipt did not survive local deletion")
	}
	destination := t.TempDir()
	if err := switchNotesFolder(destination); err != nil {
		t.Fatal(err)
	}
	receipts, err := readReceipts("sync-receipts.json")
	if err != nil || !receipts[capture.NoteID(server.URL, item.ID)] {
		t.Fatal("folder switch lost receipts")
	}
}

func serverOrigin(r *http.Request) string { return "http://" + r.Host }

func TestReceiptJournalRecoversOnlySavedNotes(t *testing.T) {
	oldDir, oldPending := dataDir, pendingSyncReceipts
	defer func() { dataDir, pendingSyncReceipts = oldDir, oldPending }()
	dataDir = t.TempDir()
	if err := writeReceipts("sync-receipts.json", map[string]bool{"earlier": true}); err != nil {
		t.Fatal(err)
	}
	if err := writeReceipts("sync-pending.json", map[string]bool{"saved": true, "trashed": true, "unsaved": true}); err != nil {
		t.Fatal(err)
	}
	if err := recoverSyncReceipts([]noteState{{ID: "saved"}}, []noteState{{ID: "trashed"}}); err != nil {
		t.Fatal(err)
	}
	receipts, err := readReceipts("sync-receipts.json")
	if err != nil {
		t.Fatal(err)
	}
	if !receipts["earlier"] || !receipts["saved"] || !receipts["trashed"] || receipts["unsaved"] {
		t.Fatalf("bad recovery: %v", receipts)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "sync-pending.json")); !os.IsNotExist(err) {
		t.Fatal("journal not cleared")
	}
}
func TestAutomaticSyncAndFolderSettingPreserveEachOther(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	disabled := false
	if err := saveAppSettings(appSettings{DataDir: "old", AutomaticSync: &disabled}); err != nil {
		t.Fatal(err)
	}
	if err := saveDataDir("new"); err != nil {
		t.Fatal(err)
	}
	settings, err := loadAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.DataDir != "new" || settings.AutomaticSync == nil || *settings.AutomaticSync {
		t.Fatal("folder change lost sync preference")
	}
	if err := saveAppSettings(appSettings{DataDir: "legacy"}); err != nil {
		t.Fatal(err)
	}
	settings, err = loadAppSettings()
	if err != nil || settings.AutomaticSync != nil {
		t.Fatal("legacy settings incompatible")
	}
}
func TestFailedReceiptSaveKeepsJournal(t *testing.T) {
	oldDir, oldPending := dataDir, pendingSyncReceipts
	defer func() { dataDir, pendingSyncReceipts = oldDir, oldPending }()
	dataDir = t.TempDir()
	pendingSyncReceipts = map[string]bool{"saved": true}
	if err := writeReceipts("sync-pending.json", pendingSyncReceipts); err != nil {
		t.Fatal(err)
	}
	// A directory in place of the receipt file makes atomic replacement fail.
	if err := os.Mkdir(filepath.Join(dataDir, "sync-receipts.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := finishSyncReceipts(); err == nil {
		t.Fatal("expected failed save")
	}
	if pendingSyncReceipts == nil {
		t.Fatal("discarded receipt state")
	}
	data, err := os.ReadFile(filepath.Join(dataDir, "sync-pending.json"))
	if err != nil {
		t.Fatal(err)
	}
	var journal map[string]bool
	if json.Unmarshal(data, &journal) != nil || !journal["saved"] {
		t.Fatal("discarded recovery journal")
	}
}
