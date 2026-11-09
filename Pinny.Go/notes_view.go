//go:build windows

package main

import (
	"fmt"
	"slices"
	stdstrings "strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	notesViewerClass = "PinnyNotesViewerGo"
	notesListID      = 211
	openNoteID       = 212
	notesRefreshID   = 1

	wmNotify            = 0x004e
	lvnItemChanged      = -101
	lvnItemActivate     = -114
	lvmDeleteAllItems   = 0x1009
	lvmGetNextItem      = 0x100c
	lvmEnsureVisible    = 0x1013
	lvmSetColumnWidth   = 0x101e
	lvmSetItemState     = 0x102b
	lvmSetExtendedStyle = 0x1036
	lvmInsertItem       = 0x104d // Unicode
	lvmInsertColumn     = 0x1061 // Unicode
	lvmSetItemText      = 0x1074 // Unicode
	lvSelected          = 2
)

type noteIndexRow struct {
	id, title, stack, created, modified string
}

type notePreview struct {
	id, title, dates, body string
}

type listColumn struct {
	mask                               uint32
	format, width                      int32
	text                               *uint16
	textMax, subItem, image, order     int32
	minWidth, defaultWidth, idealWidth int32
}

type listItem struct {
	mask                   uint32
	row, subItem           int32
	state, stateMask       uint32
	text                   *uint16
	textMax, image         int32
	param                  uintptr
	indent, groupID        int32
	columnCount            uint32
	columns, columnFormats uintptr
	group                  int32
}

type notifyHeader struct {
	source, id uintptr
	code       int32
}

type notesViewerState struct {
	hwnd, list, open, status uintptr
	title, dates, body       uintptr
	rows                     []noteIndexRow
	preview                  notePreview
	rebuilding               bool
}

var (
	notesViewer        notesViewerState
	previewLineEndings = stdstrings.NewReplacer("\r\n", "\r\n", "\r", "\r\n", "\n", "\r\n")
)

func registerNotesViewerClass(instance, largeIcon, smallIcon uintptr) error {
	wc := wndClassEx{
		size: uint32(unsafe.Sizeof(wndClassEx{})), wndProc: syscall.NewCallback(notesViewerProc),
		instance: instance, icon: largeIcon, iconSmall: smallIcon,
		cursor: call(loadCursor, 0, 32512), background: 16, // COLOR_BTNFACE + 1
		className: wide(notesViewerClass),
	}
	if call(registerClass, uintptr(unsafe.Pointer(&wc))) == 0 {
		return fmt.Errorf("could not register All notes window class")
	}
	return nil
}

func showNotesViewer() {
	if notesViewer.hwnd == 0 {
		controls := struct{ size, classes uint32 }{8, 1} // ICC_LISTVIEW_CLASSES
		if call(initCommonControls, uintptr(unsafe.Pointer(&controls))) != 0 {
			notesViewer.hwnd = call(createWindow, wsExAppWindow, uintptr(unsafe.Pointer(wide(notesViewerClass))),
				uintptr(unsafe.Pointer(wide("Pinny — All notes"))), wsOverlapped,
				0x80000000, 0x80000000, 1040, 520, 0, 0, call(getModuleHandle, 0), 0)
		}
		if notesViewer.hwnd == 0 {
			showInfo(0, "Could not open All notes.")
			return
		}
	}
	refreshNotesViewer()
	call(showWindow, notesViewer.hwnd, 9) // SW_RESTORE
	call(setForeground, notesViewer.hwnd)
	call(setFocus, notesViewer.list)
}

func notesViewerProc(hwnd, message, wp, lp uintptr) uintptr {
	switch message {
	case wmGetMinMaxInfo:
		(*minMaxInfo)(unsafe.Pointer(lp)).minTrackSize = point{780, 240}
		return 0
	case wmCreate:
		if !createNotesViewerControls(hwnd) {
			return ^uintptr(0)
		}
		return 0
	case wmSize:
		layoutNotesViewer(hwnd)
		return 0
	case wmCommand:
		if wp&0xffff == openNoteID {
			openSelectedNote()
		}
		return 0
	case wmNotify:
		notice := (*notifyHeader)(unsafe.Pointer(lp))
		if notice.source == notesViewer.list && notesViewer.body != 0 && !notesViewer.rebuilding {
			switch notice.code {
			case lvnItemChanged:
				refreshNotePreview()
			case lvnItemActivate:
				openSelectedNote()
			}
		}
		return 0
	case wmTimer:
		if wp == notesRefreshID {
			call(killTimer, hwnd, notesRefreshID)
			refreshNotesViewer()
			return 0
		}
	case wmDestroy:
		notesViewer = notesViewerState{}
		return 0
	}
	return call(defWindowProc, hwnd, message, wp, lp)
}

func createNotesViewerControls(hwnd uintptr) bool {
	instance := call(getModuleHandle, 0)
	const listStyle = wsBorder | wsTabStop | 0x800d // LVS_REPORT | LVS_SINGLESEL | LVS_SHOWSELALWAYS | LVS_NOSORTHEADER
	notesViewer.list = viewerControl("SysListView32", "", listStyle, 0, hwnd, notesListID, instance)
	notesViewer.open = viewerControl("BUTTON", "Open note", wsTabStop, 0, hwnd, openNoteID, instance)
	notesViewer.status = viewerControl("STATIC", "", 0, 0, hwnd, 0, instance)
	notesViewer.title = viewerControl("STATIC", "", 0x4080, 0, hwnd, 0, instance) // SS_ENDELLIPSIS | SS_NOPREFIX
	notesViewer.dates = viewerControl("STATIC", "", 0, 0, hwnd, 0, instance)
	notesViewer.body = viewerControl("EDIT", "", wsBorder|wsVScroll|wsTabStop|esMultiline|esAutoVScroll|esReadOnly, 0, hwnd, 0, instance)
	for _, control := range []uintptr{notesViewer.list, notesViewer.open, notesViewer.status, notesViewer.title, notesViewer.dates, notesViewer.body} {
		if control == 0 {
			return false
		}
		call(sendMessage, control, wmSetFont, font, 1)
	}
	call(sendMessage, notesViewer.list, lvmSetExtendedStyle, 0x20, 0x20) // LVS_EX_FULLROWSELECT
	call(sendMessage, notesViewer.body, emLimitText, 0x7fffffff, 0)
	call(sendMessage, notesViewer.body, emSetMargins, 3, 8|(8<<16))
	for index, label := range []string{"Note", "Stack", "Created", "Modified"} {
		column := listColumn{mask: 0x0006, width: 140, text: wide(label)} // LVCF_WIDTH | LVCF_TEXT
		if call(sendMessage, notesViewer.list, lvmInsertColumn, uintptr(index), uintptr(unsafe.Pointer(&column))) == ^uintptr(0) {
			return false
		}
	}
	layoutNotesViewer(hwnd)
	return true
}

func layoutNotesViewer(hwnd uintptr) {
	if notesViewer.list == 0 {
		return
	}
	var bounds rect
	call(getClientRect, hwnd, uintptr(unsafe.Pointer(&bounds)))
	const margin, gap, footer, previewHeader = int32(12), int32(14), int32(38), int32(68)
	width := max(bounds.right-2*margin-gap, 2)
	height := max(bounds.bottom-2*margin-footer, 1)
	listWidth := min(max(width*60/100, 475), width-1)
	previewX, previewWidth := margin+listWidth+gap, width-listWidth
	call(moveWindow, notesViewer.list, uintptr(margin), uintptr(margin), uintptr(listWidth), uintptr(height), 1)
	call(moveWindow, notesViewer.title, uintptr(previewX), uintptr(margin), uintptr(previewWidth), 22, 1)
	call(moveWindow, notesViewer.dates, uintptr(previewX), uintptr(margin+24), uintptr(previewWidth), 38, 1)
	call(moveWindow, notesViewer.body, uintptr(previewX), uintptr(margin+previewHeader), uintptr(previewWidth), uintptr(max(height-previewHeader, 1)), 1)
	call(moveWindow, notesViewer.status, uintptr(margin), uintptr(margin+height+12), 220, 22, 1)
	call(moveWindow, notesViewer.open, uintptr(max(margin, bounds.right-margin-100)), uintptr(margin+height+8), 100, 28, 1)
	columnWidth := max(listWidth-int32(call(getSystemMetrics, 2))-3, 4) // SM_CXVSCROLL
	dateWidth := min(144, columnWidth/3)
	labelWidth := columnWidth - 2*dateWidth
	stackWidth := min(max(labelWidth/3, 80), labelWidth/2, 150)
	for index, pixels := range []int32{labelWidth - stackWidth, stackWidth, dateWidth, dateWidth} {
		call(sendMessage, notesViewer.list, lvmSetColumnWidth, uintptr(index), uintptr(pixels))
	}
}

func queueNotesViewerRefresh() {
	if notesViewer.hwnd != 0 && !quitting {
		call(setTimer, notesViewer.hwnd, notesRefreshID, 180, 0)
	}
}

func noteDateLabel(date *time.Time) string {
	if date == nil || date.IsZero() {
		return "Unknown"
	}
	return date.Local().Format("02 Jan 2006 15:04")
}

func makeNoteIndex(states []noteState) []noteIndexRow {
	slices.SortStableFunc(states, func(a, b noteState) int {
		if a.ModifiedAt == nil {
			if b.ModifiedAt == nil {
				return 0
			}
			return 1
		}
		if b.ModifiedAt == nil {
			return -1
		}
		return b.ModifiedAt.Compare(*a.ModifiedAt)
	})
	rows := make([]noteIndexRow, 0, len(states))
	for _, state := range states {
		row := noteIndexRow{id: state.ID, title: stackNoteLabel(state), stack: "Standalone",
			created: noteDateLabel(state.CreatedAt), modified: noteDateLabel(state.ModifiedAt)}
		if state.Stack != nil {
			row.stack = state.Stack.Name
		}
		rows = append(rows, row)
	}
	return rows
}

func selectedNoteIndex() int {
	return int(call(sendMessage, notesViewer.list, lvmGetNextItem, ^uintptr(0), lvSelected))
}

func refreshNotesViewer() {
	states, err := snapshotNotes(0)
	if err != nil {
		call(setWindowText, notesViewer.status, uintptr(unsafe.Pointer(wide("Could not refresh notes."))))
		return
	}
	rows := makeNoteIndex(states)
	count := fmt.Sprintf("%d notes", len(rows))
	if len(rows) == 1 {
		count = "1 note"
	}
	call(setWindowText, notesViewer.status, uintptr(unsafe.Pointer(wide(count))))
	if slices.Equal(rows, notesViewer.rows) {
		refreshNotePreview()
		return
	}
	selectedID := ""
	if index := selectedNoteIndex(); index >= 0 && index < len(notesViewer.rows) {
		selectedID = notesViewer.rows[index].id
	}
	notesViewer.rows = rows
	notesViewer.rebuilding = true
	list := notesViewer.list
	call(sendMessage, list, 0x000b, 0, 0) // WM_SETREDRAW
	call(sendMessage, list, lvmDeleteAllItems, 0, 0)
	selected := 0
	for index, row := range rows {
		item := listItem{mask: 1, row: int32(index), text: wide(row.title)} // LVIF_TEXT
		call(sendMessage, list, lvmInsertItem, 0, uintptr(unsafe.Pointer(&item)))
		for column, value := range []string{row.stack, row.created, row.modified} {
			item.subItem, item.text = int32(column+1), wide(value)
			call(sendMessage, list, lvmSetItemText, uintptr(index), uintptr(unsafe.Pointer(&item)))
		}
		if row.id == selectedID {
			selected = index
		}
	}
	if len(rows) > 0 {
		const selectedAndFocused = lvSelected | 1
		item := listItem{state: selectedAndFocused, stateMask: selectedAndFocused}
		call(sendMessage, list, lvmSetItemState, uintptr(selected), uintptr(unsafe.Pointer(&item)))
		call(sendMessage, list, lvmEnsureVisible, uintptr(selected), 0)
	}
	call(sendMessage, list, 0x000b, 1, 0)
	call(invalidateRect, list, 0, 1)
	notesViewer.rebuilding = false
	refreshNotePreview()
}

// A negative stack index denotes the note already loaded in the editor.
func selectedNoteWindow() (uintptr, int) {
	index := selectedNoteIndex()
	if index < 0 || index >= len(notesViewer.rows) {
		return 0, -1
	}
	id := notesViewer.rows[index].id
	for _, hwnd := range windowOrder {
		n := windows[hwnd]
		if n.id == id {
			return hwnd, -1
		}
		if n.stack != nil {
			if index := slices.IndexFunc(n.stack.notes, func(state noteState) bool { return state.ID == id }); index >= 0 {
				return hwnd, index
			}
		}
	}
	return 0, -1
}

func refreshNotePreview() {
	hwnd, index := selectedNoteWindow()
	preview := notePreview{title: "Select a note"}
	if hwnd != 0 {
		n := windows[hwnd]
		var state noteState
		if index >= 0 {
			state = n.stack.notes[index]
		} else {
			state = activeNoteState(n)
		}
		preview = notePreview{id: state.ID, title: stackNoteLabel(state), body: state.Text,
			dates: "Created: " + noteDateLabel(state.CreatedAt) + "\r\nModified: " + noteDateLabel(state.ModifiedAt)}
	}
	old := notesViewer.preview
	if preview.title != old.title {
		call(setWindowText, notesViewer.title, uintptr(unsafe.Pointer(wide(preview.title))))
	}
	if preview.dates != old.dates {
		call(setWindowText, notesViewer.dates, uintptr(unsafe.Pointer(wide(preview.dates))))
	}
	if preview.id != old.id || preview.body != old.body {
		body := previewLineEndings.Replace(preview.body)
		call(setWindowText, notesViewer.body, uintptr(unsafe.Pointer(wide(body))))
		call(sendMessage, notesViewer.body, emSetSel, 0, 0)
	}
	notesViewer.preview = preview
	call(enableWindow, notesViewer.open, boolToUintptr(hwnd != 0))
}

func openSelectedNote() {
	hwnd, index := selectedNoteWindow()
	if hwnd == 0 {
		refreshNotesViewer()
		return
	}
	n := windows[hwnd]
	if index >= 0 {
		selectStackNote(hwnd, n, index)
	}
	call(showWindow, hwnd, 9) // SW_RESTORE
	call(setForeground, hwnd)
	call(setFocus, n.edit)
}
