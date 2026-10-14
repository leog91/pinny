//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"pinny-go/internal/capture"
)

const (
	syncNowID       = 30
	automaticSyncID = 31
	configureSyncID = 32
)

type syncEvent struct {
	items    []capture.Item
	reply    chan error
	err      error
	finished bool
	manual   bool
}

var (
	syncConfig               capture.Config
	automaticSync            = true
	syncBusy                 bool // UI-thread owned; no Win32 state is accessed by the network goroutine.
	syncTimer, syncPollTimer uintptr
	syncEvents               = make(chan syncEvent, 1)
	syncCancel               context.CancelFunc
	pendingSyncReceipts      map[string]bool
)

func initializeSync() {
	if !persistenceEnabled {
		return
	}
	if settings, err := loadAppSettings(); err == nil && settings.AutomaticSync != nil {
		automaticSync = *settings.AutomaticSync
	}
	dir, err := defaultDataDir()
	if err != nil {
		return
	}
	data, err := os.ReadFile(filepath.Join(dir, "sync.json"))
	if err != nil {
		return
	}
	var config capture.Config
	if len(data) <= 8192 && json.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &config) == nil {
		syncConfig, _ = config.Normalize()
	}
}
func appendSyncMenu(n *note) {
	call(appendMenu, n.menu, mfSeparator, 0, 0)
	call(appendMenu, n.menu, 0, syncNowID, uintptr(unsafe.Pointer(wide("Sync now"))))
	call(appendMenu, n.menu, 0, configureSyncID, uintptr(unsafe.Pointer(wide("Configure sync..."))))
	call(appendMenu, n.menu, 0, automaticSyncID, uintptr(unsafe.Pointer(wide("Automatic sync"))))
	updateSyncMenu(n)
}
func updateSyncMenu(n *note) {
	flags := uintptr(0)
	if automaticSync {
		flags = mfChecked
	}
	call(checkMenuItem, n.menu, automaticSyncID, flags)
}
func startAutomaticSync() {
	if syncTimer != 0 {
		call(killTimer, 0, syncTimer)
		syncTimer = 0
	}
	if persistenceEnabled && automaticSync && syncConfig.Configured() {
		syncTimer = call(setTimer, 0, 0, uintptr(capture.Interval/time.Millisecond), 0)
		startSync(false)
	}
}
func stopSync() {
	if syncCancel != nil {
		syncCancel()
	}
	if syncTimer != 0 {
		call(killTimer, 0, syncTimer)
	}
	if syncPollTimer != 0 {
		call(killTimer, 0, syncPollTimer)
	}
}
func toggleAutomaticSync(owner uintptr) {
	settings, err := loadAppSettings()
	if err != nil {
		showOperationError(owner, "Automatic sync", errors.New("could not read settings"))
		return
	}
	value := !automaticSync
	settings.AutomaticSync = &value
	if err := saveAppSettings(settings); err != nil {
		showOperationError(owner, "Automatic sync", errors.New("could not save settings"))
		return
	}
	automaticSync = value
	for _, n := range windows {
		updateSyncMenu(n)
	}
	if syncTimer != 0 {
		call(killTimer, 0, syncTimer)
		syncTimer = 0
	}
	if automaticSync && syncConfig.Configured() {
		syncTimer = call(setTimer, 0, 0, uintptr(capture.Interval/time.Millisecond), 0)
	}
	// Disabling stops future triggers; an already running delivery finishes safely.
}
func configureSync(owner uintptr) {
	if !persistenceEnabled {
		showInfo(owner, "Sync is unavailable in --notes mode.")
		return
	}
	if syncBusy {
		showInfo(owner, "Wait for the current sync to finish before changing configuration.")
		return
	}
	path, err := choosePath(owner, "Choose a sync configuration JSON file", "", false, false)
	if err != nil {
		showInfo(owner, "Could not open the configuration file.")
		return
	}
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	var config capture.Config
	if err != nil || len(data) > 8192 || json.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &config) != nil {
		showInfo(owner, "Invalid sync configuration file.")
		return
	}
	config, err = config.Normalize()
	if err != nil {
		showInfo(owner, err.Error())
		return
	}
	dir, err := defaultDataDir()
	if err != nil {
		showInfo(owner, "Could not find the settings folder.")
		return
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		showInfo(owner, "Could not save sync configuration.")
		return
	}
	encoded, _ := json.MarshalIndent(config, "", "  ")
	if err := writeAtomic(filepath.Join(dir, "sync.json"), encoded); err != nil {
		showInfo(owner, "Could not save sync configuration.")
		return
	}
	syncConfig = config
	startAutomaticSync()
	showInfo(owner, "Sync configured. The app credential is stored in your local Pinny settings folder. Use Sync now to collect pending items.")
}
func startSync(manual bool) {
	if !persistenceEnabled || !syncConfig.Configured() {
		if manual {
			showInfo(0, "Configure sync with an endpoint and desktop app credential first.")
		}
		return
	}
	if syncBusy {
		if manual {
			showInfo(0, "Sync is already running.")
		}
		return
	}
	// The timer only drains a channel on the UI thread; network I/O runs elsewhere.
	syncPollTimer = call(setTimer, 0, 0, 100, 0)
	if syncPollTimer == 0 {
		if manual {
			showInfo(0, "Could not start sync.")
		}
		return
	}
	syncBusy = true
	config := syncConfig
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	syncCancel = cancel
	go func() {
		err := (capture.Client{Config: config}).Sync(ctx, func(items []capture.Item) error {
			reply := make(chan error, 1)
			select {
			case syncEvents <- syncEvent{items: items, reply: reply}:
			case <-ctx.Done():
				return ctx.Err()
			}
			select {
			case err := <-reply:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		select {
		case syncEvents <- syncEvent{err: err, finished: true, manual: manual}:
		case <-ctx.Done():
			// Keep completion notification even when canceled by the deadline.
			syncEvents <- syncEvent{err: errors.New("sync timed out; retry later"), finished: true, manual: manual}
		}
		cancel()
	}()
}
func handleSyncMessage(message msg) bool {
	if message.hwnd != 0 || message.message != wmTimer {
		return false
	}
	if syncTimer != 0 && message.wParam == syncTimer {
		if automaticSync {
			startSync(false)
		}
		return true
	}
	if syncPollTimer == 0 || message.wParam != syncPollTimer {
		return false
	}
	select {
	case event := <-syncEvents:
		if event.finished {
			syncBusy = false
			call(killTimer, 0, syncPollTimer)
			syncPollTimer = 0
			if event.manual {
				if event.err != nil {
					showInfo(0, event.err.Error())
				} else {
					showInfo(0, "Sync completed. Incoming items are saved locally.")
				}
			}
		} else {
			event.reply <- applyIncoming(syncConfig.Endpoint, event.items)
		}
	default:
	}
	return true
}

func readReceipts(name string) (map[string]bool, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, name))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	var receipts map[string]bool
	if err := json.Unmarshal(data, &receipts); err != nil {
		return nil, errors.New("invalid sync receipt file")
	}
	if receipts == nil {
		receipts = map[string]bool{}
	}
	return receipts, nil
}
func writeReceipts(name string, receipts map[string]bool) error {
	data, err := json.Marshal(receipts)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dataDir, name), data)
}

// A write-ahead ID journal closes the crash gap between notes.json and receipts.
// Recovery only marks IDs present in durably saved notes/Trash; unsaved items stay pending.
func recoverSyncReceipts(notes, deleted []noteState) error {
	journal, err := readReceipts("sync-pending.json")
	if err != nil {
		return err
	}
	if len(journal) == 0 {
		return nil
	}
	receipts, err := readReceipts("sync-receipts.json")
	if err != nil {
		return err
	}
	for _, list := range [][]noteState{notes, deleted} {
		for _, note := range list {
			if journal[note.ID] {
				receipts[note.ID] = true
			}
		}
	}
	if err := writeReceipts("sync-receipts.json", receipts); err != nil {
		return err
	}
	// Receipts are already durable; a stale journal is safe to recover again.
	_ = os.Remove(filepath.Join(dataDir, "sync-pending.json"))
	return nil
}
func finishSyncReceipts() error {
	if pendingSyncReceipts == nil {
		return nil
	}
	if err := writeReceipts("sync-receipts.json", pendingSyncReceipts); err != nil {
		return err
	}
	pendingSyncReceipts = nil
	// Stale journal removal is harmless: recovery unions receipts, never removes them.
	_ = os.Remove(filepath.Join(dataDir, "sync-pending.json"))
	return nil
}
func applyIncoming(endpoint string, items []capture.Item) error {
	if quitting {
		return errors.New("closing")
	}
	if pendingSyncReceipts != nil {
		if err := saveAllExcept(0); err != nil {
			return err
		}
	}
	receipts, err := readReceipts("sync-receipts.json")
	if err != nil {
		return err
	}
	notes, err := snapshotNotes(0)
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	for _, list := range [][]noteState{notes, trash} {
		for _, n := range list {
			existing[n.ID] = true
		}
	}
	fresh, next := capture.Plan(endpoint, items, receipts, existing)
	journal := map[string]bool{}
	for _, item := range items {
		journal[capture.NoteID(endpoint, item.ID)] = true
	}
	if err := writeReceipts("sync-pending.json", journal); err != nil {
		return err
	}
	// Never consider a failed window creation delivered. On partial creation, save
	// receipts only for successful editors; the entire cloud batch stays pending.
	for _, item := range fresh {
		if err := showNote(initialNote{id: capture.NoteID(endpoint, item.ID), text: item.NoteText(), title: item.Title,
			x: -2147483648, y: -2147483648, width: 320, height: 320, theme: "Light"}); err != nil {
			for id := range next {
				if !receipts[id] && !existing[id] {
					delete(next, id)
				}
			}
			pendingSyncReceipts = next
			_ = saveAllExcept(0)
			return err
		}
		existing[capture.NoteID(endpoint, item.ID)] = true
	}
	pendingSyncReceipts = next
	stopSaveTimer()
	return saveAllExcept(0)
}
