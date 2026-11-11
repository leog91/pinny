//go:build windows

package main

import (
	"reflect"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestStacksWithLiveNotes(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	oldDir, oldPersistence, oldQuitting := dataDir, persistenceEnabled, quitting
	oldWindows, oldOrder, oldPalettes, oldTrash := windows, windowOrder, palettes, trash
	dataDir, persistenceEnabled, quitting = t.TempDir(), true, false
	windows, windowOrder, palettes, trash = map[uintptr]*note{}, nil, map[string]palette{}, nil
	wc := wndClassEx{size: uint32(unsafe.Sizeof(wndClassEx{})), wndProc: syscall.NewCallback(wndProc),
		instance: call(getModuleHandle, 0), className: wide(windowClass)}
	if call(registerClass, uintptr(unsafe.Pointer(&wc))) == 0 {
		t.Fatal("could not register test note class")
	}
	p := makePalette(255, 255, 255, 244, 245, 247, 200, 205, 211, 225, 229, 235, 32, 35, 40, 85, 91, 99)
	palettes["Light"] = p
	defer func() {
		stopSaveTimer()
		for _, hwnd := range append([]uintptr(nil), windowOrder...) {
			call(destroyWindow, hwnd)
		}
		for _, brush := range []uintptr{p.surface, p.header, p.border, p.hover} {
			call(deleteObject, brush)
		}
		call(user32.NewProc("UnregisterClassW"), uintptr(unsafe.Pointer(wide(windowClass))), wc.instance)
		dataDir, persistenceEnabled, quitting = oldDir, oldPersistence, oldQuitting
		windows, windowOrder, palettes, trash = oldWindows, oldOrder, oldPalettes, oldTrash
	}()

	for _, id := range []string{"one", "two", "three", "loose"} {
		if err := showSavedNote(noteState{ID: id, Title: id, Text: id + " body", Theme: "Light",
			Left: 80, Top: 80, Width: 400, Height: 320}); err != nil {
			t.Fatal(err)
		}
	}
	first, second, third, loose := windowOrder[0], windowOrder[1], windowOrder[2], windowOrder[3]
	n := windows[first]
	editor := n.edit
	command := func(hwnd, id uintptr) { call(sendMessage, hwnd, wmCommand, id, 0) }
	command(first, createStackID)
	for _, hwnd := range []uintptr{second, third} {
		refreshStackMenu(hwnd, windows[hwnd])
		command(hwnd, firstStackMoveID)
	}
	if len(windowOrder) != 2 || n.stack == nil || n.edit != editor || windows[loose].stack != nil {
		t.Fatal("grouping did not preserve one stack editor and the standalone window")
	}
	assertOrder := func(want ...string) {
		t.Helper()
		var got []string
		for _, state := range n.stack.notes {
			got = append(got, state.ID)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("stack order: got %v, want %v", got, want)
		}
	}
	assertOrder("one", "two", "three")
	// Exercise the actual header hit targets with a stack and a pinned note.
	togglePin(loose, windows[loose])
	beforeMinimize, err := snapshotNotes(0)
	if err != nil {
		t.Fatal(err)
	}
	clickHeader := func(hwnd uintptr, offset int32) {
		t.Helper()
		var bounds rect
		call(getClientRect, hwnd, uintptr(unsafe.Pointer(&bounds)))
		point := packedPoint(point{bounds.right - offset, 16})
		call(sendMessage, hwnd, wmLButtonDown, 1, point)
		call(sendMessage, hwnd, wmLButtonUp, 0, point)
	}
	clickHeader(first, 162) // Minimize this stack only.
	if call(isIconic, first) == 0 || call(isIconic, loose) != 0 {
		t.Fatal("single-note minimize affected the wrong windows")
	}
	clickHeader(loose, 198) // Minimize all, including an already minimized stack.
	for _, hwnd := range windowOrder {
		if call(isIconic, hwnd) == 0 {
			t.Fatal("minimize all left a note visible")
		}
	}
	afterMinimize, err := snapshotNotes(0)
	if err != nil || !reflect.DeepEqual(beforeMinimize, afterMinimize) {
		t.Fatalf("minimizing changed saved notes or geometry: %v\nbefore: %+v\nafter: %+v", err, beforeMinimize, afterMinimize)
	}
	if err := saveAllExcept(0); err != nil {
		t.Fatal(err)
	}
	for _, hwnd := range windowOrder {
		call(showWindow, hwnd, 9) // SW_RESTORE
	}
	afterRestore, err := snapshotNotes(0)
	if err != nil || !reflect.DeepEqual(beforeMinimize, afterRestore) {
		t.Fatal("restoring did not preserve note contents and geometry")
	}
	call(setWindowText, n.edit, uintptr(unsafe.Pointer(wide("edited three"))))
	var client rect
	call(getClientRect, first, uintptr(unsafe.Pointer(&client)))
	up := packedPoint(point{client.right - baseHeaderWidth - 36, 16})
	down := packedPoint(point{client.right - baseHeaderWidth - 12, 16})
	for _, arrow := range []uintptr{up, down} {
		call(sendMessage, first, wmLButtonDown, 1, arrow)
		call(sendMessage, first, wmLButtonUp, 0, arrow)
		if arrow == up {
			assertOrder("one", "three", "two")
		}
	}
	assertOrder("one", "two", "three")
	listPoint := func(index int) uintptr {
		t.Helper()
		var bounds rect
		if call(sendMessage, n.stack.list, lbGetItemRect, uintptr(index), uintptr(unsafe.Pointer(&bounds))) == ^uintptr(0) {
			t.Fatalf("could not read list row %d", index)
		}
		return packedPoint(point{8, (bounds.top + bounds.bottom) / 2})
	}
	from, to := listPoint(2), listPoint(0)
	call(sendMessage, n.stack.list, wmLButtonDown, 1, from)
	call(sendMessage, n.stack.list, wmMouseMove, 1, to)
	call(sendMessage, n.stack.list, wmLButtonUp, 0, to)
	assertOrder("three", "one", "two")
	if n.id != "three" || editorText(n.edit) != "edited three" {
		t.Fatal("reordering changed the selected note or its edits")
	}
	// Switch to another note through the list notification.
	call(sendMessage, n.stack.list, lbSetCurSel, 1, 0)
	command(first, stackListID|1<<16)
	if n.id != "one" || editorText(n.edit) != "one body" || n.edit != editor {
		t.Fatal("switching did not load the selected note into the same editor")
	}
	command(first, popStackNoteID)
	assertOrder("three", "two")
	popped := windowOrder[len(windowOrder)-1]
	if other := windows[popped]; other.stack != nil || other.id != "one" || other.returnTo == nil {
		t.Fatal("pop out did not produce a standalone note with a return position")
	}
	command(popped, returnStackNoteID)
	assertOrder("three", "one", "two")
	command(first, trashID)
	assertOrder("three", "two")
	if len(trash) != 1 || trash[0].ID != "one" {
		t.Fatalf("unexpected Trash: %v", trash)
	}
	restoreTrashAt(0)
	command(windowOrder[len(windowOrder)-1], returnStackNoteID)
	assertOrder("three", "one", "two")
	command(first, renameStackID)
	call(setWindowText, n.titleEdit, uintptr(unsafe.Pointer(wide("Work"))))
	handleEditorMessage(msg{hwnd: n.titleEdit, message: 0x0100, wParam: 13})
	if err := saveAllExcept(0); err != nil {
		t.Fatal(err)
	}
	saved, err := loadNotes(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, hwnd := range append([]uintptr(nil), windowOrder...) {
		call(destroyWindow, hwnd)
	}
	if err := showSavedNotes(saved); err != nil {
		t.Fatal(err)
	}
	if len(windowOrder) != 2 {
		t.Fatalf("reopened %d windows, want 2", len(windowOrder))
	}
	n = windows[windowOrder[0]]
	assertOrder("three", "one", "two")
	if n.stack.name != "Work" || n.id != "one" || n.stack.notes[0].Text != "edited three" {
		t.Fatal("reload lost the stack name, selected note, or an inactive note's edits")
	}
	if err := addImportedNotes(notesBackup{Version: 1, Notes: saved}); err != nil {
		t.Fatal(err)
	}
	if len(windowOrder) != 4 || windows[windowOrder[2]].stack.id == n.stack.id {
		t.Fatal("import did not restore the stack as a separate group")
	}
}

func TestAllNotesViewerAndDates(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	oldWindows, oldOrder, oldPersistence, oldQuitting := windows, windowOrder, persistenceEnabled, quitting
	windows, windowOrder, persistenceEnabled, quitting = map[uintptr]*note{}, nil, false, false
	instance := call(getModuleHandle, 0)
	wc := wndClassEx{size: uint32(unsafe.Sizeof(wndClassEx{})), wndProc: syscall.NewCallback(wndProc),
		instance: instance, className: wide(windowClass)}
	if call(registerClass, uintptr(unsafe.Pointer(&wc))) == 0 {
		t.Fatal("could not register test note class")
	}
	if err := registerNotesViewerClass(instance, 0, 0); err != nil {
		t.Fatal(err)
	}
	defer func() {
		call(destroyWindow, notesViewer.hwnd)
		for _, hwnd := range append([]uintptr(nil), windowOrder...) {
			call(destroyWindow, hwnd)
		}
		for _, class := range []string{windowClass, notesViewerClass} {
			call(user32.NewProc("UnregisterClassW"), uintptr(unsafe.Pointer(wide(class))), instance)
		}
		windows, windowOrder, persistenceEnabled, quitting = oldWindows, oldOrder, oldPersistence, oldQuitting
	}()
	created := time.Date(2026, 1, 9, 12, 0, 0, 0, time.UTC)
	member := &stackMembership{ID: "work", Name: "Work"}
	states := []noteState{
		{ID: "a", Title: "First", Text: "First body", Stack: member, noteDates: initialNoteDates(created)},
		{ID: "b", Title: "Second", Text: "Line one\nLine two", Stack: member},
		{ID: "legacy", Title: "Older note"},
	}
	if err := showSavedNotes(states); err != nil {
		t.Fatal(err)
	}
	hwnd := windowOrder[0]
	n := windows[hwnd]
	call(sendMessage, hwnd, wmCommand, allNotesID, 0)
	if notesViewer.hwnd == 0 || call(sendMessage, notesViewer.list, 0x1004, 0, 0) != 3 { // LVM_GETITEMCOUNT
		t.Fatal("All notes did not show one native row per note")
	}
	cell := func(row, column int) string {
		buffer := make([]uint16, 128)
		item := listItem{subItem: int32(column), text: &buffer[0], textMax: int32(len(buffer))}
		call(sendMessage, notesViewer.list, 0x1073, uintptr(row), uintptr(unsafe.Pointer(&item))) // LVM_GETITEMTEXTW
		return syscall.UTF16ToString(buffer)
	}
	header := call(sendMessage, notesViewer.list, 0x101f, 0, 0) // LVM_GETHEADER
	if call(user32.NewProc("IsWindowVisible"), header) == 0 {
		t.Fatal("All notes column headers are hidden")
	}
	if call(sendMessage, header, 0x1200, 0, 0) != 4 { // HDM_GETITEMCOUNT
		t.Fatal("All notes did not show four columns")
	}
	date := noteDateLabel(&created)
	if cell(0, 0) != "First" || cell(0, 1) != "Work" || cell(0, 2) != date || cell(0, 3) != date ||
		cell(2, 1) != "Standalone" || cell(2, 2) != "Unknown" || cell(2, 3) != "Unknown" {
		t.Fatal("native list did not show note names, stack membership, and both dates")
	}
	checkPreview := func(title, dates, body string) {
		t.Helper()
		if got := editorText(notesViewer.title); got != title {
			t.Fatalf("preview title: got %q, want %q", got, title)
		}
		if got := editorText(notesViewer.dates); got != dates {
			t.Fatalf("preview dates: got %q, want %q", got, dates)
		}
		if got := editorText(notesViewer.body); got != body {
			t.Fatalf("preview text: got %q, want %q", got, body)
		}
	}
	checkPreview("First", "Created: "+noteDateLabel(&created)+"\r\nModified: "+noteDateLabel(&created), "First body")
	var listBounds, previewBounds rect
	call(getWindowRect, notesViewer.list, uintptr(unsafe.Pointer(&listBounds)))
	call(getWindowRect, notesViewer.body, uintptr(unsafe.Pointer(&previewBounds)))
	if previewBounds.left <= listBounds.right || previewBounds.bottom != listBounds.bottom {
		t.Fatal("the preview is not laid out beside the list")
	}
	item := listItem{state: lvSelected, stateMask: lvSelected}
	call(sendMessage, notesViewer.list, lvmSetItemState, 1, uintptr(unsafe.Pointer(&item)))
	checkPreview("Second", "Created: Unknown\r\nModified: Unknown", "Line one\r\nLine two")
	if n.id != "a" {
		t.Fatal("previewing a stack note changed the active editor")
	}
	call(sendMessage, notesViewer.hwnd, wmCommand, openNoteID, 0)
	if n.id != "b" || n.dates.ModifiedAt != nil {
		t.Fatal("opening an inactive stack note did not select it, or recorded an edit")
	}
	call(sendMessage, n.edit, emReplaceSel, 1, uintptr(unsafe.Pointer(wide("Edited"))))
	if n.dates.CreatedAt != nil || n.dates.ModifiedAt == nil {
		t.Fatal("editing did not record a modification while preserving the unknown creation date")
	}
	modified := *n.dates.ModifiedAt
	moveStackNote(hwnd, n, 0)
	refreshNotesViewer()
	if cell(0, 0) != "Second" || cell(0, 2) != "Unknown" || cell(0, 3) != noteDateLabel(&modified) {
		t.Fatal("All notes did not sort the edited note first and refresh its modification date")
	}
	checkPreview("Second", "Created: Unknown\r\nModified: "+noteDateLabel(&modified), "EditedLine one\r\nLine two")
	if !n.dates.ModifiedAt.Equal(modified) {
		t.Fatal("reordering changed the modification timestamp")
	}
	call(sendMessage, hwnd, wmCommand, newID, 0)
	if n.dates.CreatedAt == nil || n.dates.ModifiedAt == nil || !n.dates.CreatedAt.Equal(*n.dates.ModifiedAt) {
		t.Fatal("new note did not start with matching creation and modification dates")
	}
	beginRename(hwnd, n)
	call(setWindowText, n.titleEdit, uintptr(unsafe.Pointer(wide("Renamed note"))))
	finishRename(hwnd, n, true)
	if n.dates.ModifiedAt.Before(*n.dates.CreatedAt) || n.title != "Renamed note" {
		t.Fatal("renaming did not retain the creation date")
	}
}
