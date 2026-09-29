//go:build windows

package main

import (
	"fmt"
	"runtime"
	stdstrings "strings"
	"syscall"
	"unsafe"
)

const (
	trashViewerClass = "PinnyTrashViewerGo"
	wmCreate         = 0x0001
	wmGetMinMaxInfo  = 0x0024
	wsOverlapped     = 0x00CF0000
	wsBorder         = 0x00800000
	wsTabStop        = 0x00010000
	bsDefPushButton  = 0x0001
	esReadOnly       = 0x0800
	lbsNotify        = 0x0001
	lbAddString      = 0x0180
	lbResetContent   = 0x0184
	lbSetCurSel      = 0x0186
	lbGetCurSel      = 0x0188
	emSetSel         = 0x00B1
	trashListID      = 201
	trashRestoreID   = 202
	trashCloseID     = 203
	trashDeleteID    = 204
	trashEmptyID     = 205
)

type minMaxInfo struct {
	reserved, maxSize, maxPosition, minTrackSize, maxTrackSize point
}

var (
	trashViewer   uintptr
	trashList     uintptr
	trashPreview  uintptr
	trashRestore  uintptr
	trashClose    uintptr
	trashDelete   uintptr
	trashEmpty    uintptr
	setWindowText = user32.NewProc("SetWindowTextW")
	enableWindow  = user32.NewProc("EnableWindow")
)

func registerTrashViewerClass(instance, largeIcon, smallIcon uintptr) error {
	wc := wndClassEx{
		size:       uint32(unsafe.Sizeof(wndClassEx{})),
		wndProc:    syscall.NewCallback(trashViewerProc),
		instance:   instance,
		icon:       largeIcon,
		iconSmall:  smallIcon,
		cursor:     call(loadCursor, 0, 32512),
		background: 16, // COLOR_BTNFACE + 1
		className:  wide(trashViewerClass),
	}
	if call(registerClass, uintptr(unsafe.Pointer(&wc))) == 0 {
		return fmt.Errorf("could not register Trash viewer class")
	}
	return nil
}

func showTrashViewer() {
	if !persistenceEnabled {
		showInfo(0, "Trash is unavailable in --notes mode.")
		return
	}
	if trashViewer != 0 {
		call(showWindow, trashViewer, 9) // SW_RESTORE
		call(setForeground, trashViewer)
		return
	}
	hwnd := call(createWindow, wsExAppWindow, uintptr(unsafe.Pointer(wide(trashViewerClass))),
		uintptr(unsafe.Pointer(wide("Pinny — Trash"))), wsOverlapped|wsVisible,
		0x80000000, 0x80000000, 640, 440, 0, 0, call(getModuleHandle, 0), 0)
	if hwnd == 0 {
		call(messageBox, 0, uintptr(unsafe.Pointer(wide("Could not open Trash viewer."))),
			uintptr(unsafe.Pointer(wide("Pinny"))), 0x10)
		return
	}
	trashViewer = hwnd
	refreshTrashViewer()
	call(showWindow, hwnd, 5)
	call(updateWindow, hwnd)
	call(setForeground, hwnd)
	call(setFocus, trashList)
}

func trashViewerProc(hwnd, message, wp, lp uintptr) uintptr {
	switch message {
	case wmGetMinMaxInfo:
		limits := (*minMaxInfo)(unsafe.Pointer(lp))
		limits.minTrackSize = point{480, 260}
		return 0
	case wmCreate:
		instance := call(getModuleHandle, 0)
		trashList = viewerControl("LISTBOX", "", wsBorder|wsVScroll|wsTabStop|lbsNotify, 0, hwnd, trashListID, instance)
		trashPreview = viewerControl("EDIT", "", wsBorder|wsVScroll|esMultiline|esAutoVScroll|esReadOnly|wsTabStop, 0, hwnd, 0, instance)
		trashRestore = viewerControl("BUTTON", "Restore note", wsTabStop|bsDefPushButton, 0, hwnd, trashRestoreID, instance)
		trashClose = viewerControl("BUTTON", "Close", wsTabStop, 0, hwnd, trashCloseID, instance)
		trashDelete = viewerControl("BUTTON", "Delete note...", wsTabStop, 0, hwnd, trashDeleteID, instance)
		trashEmpty = viewerControl("BUTTON", "Empty Trash...", wsTabStop, 0, hwnd, trashEmptyID, instance)
		if trashList == 0 || trashPreview == 0 || trashRestore == 0 || trashClose == 0 || trashDelete == 0 || trashEmpty == 0 {
			return ^uintptr(0) // Fail WM_CREATE.
		}
		for _, control := range []uintptr{trashList, trashPreview, trashRestore, trashClose, trashDelete, trashEmpty} {
			call(sendMessage, control, wmSetFont, font, 1)
		}
		layoutTrashViewer(hwnd)
		return 0
	case wmSize:
		layoutTrashViewer(hwnd)
		return 0
	case wmCommand:
		id, notification := wp&0xffff, (wp>>16)&0xffff
		switch {
		case id == trashListID && notification == 1: // LBN_SELCHANGE
			showSelectedTrashPreview()
		case id == trashRestoreID:
			if index := selectedTrashIndex(); index >= 0 {
				restoreTrashAt(index)
			}
		case id == trashCloseID:
			call(destroyWindow, hwnd)
		case id == trashDeleteID:
			deleteSelectedTrash()
		case id == trashEmptyID:
			emptyTrash()
		}
		return 0
	case wmClose:
		call(destroyWindow, hwnd)
		return 0
	case wmDestroy:
		trashViewer, trashList, trashPreview = 0, 0, 0
		trashRestore, trashClose, trashDelete, trashEmpty = 0, 0, 0, 0
		return 0
	}
	return call(defWindowProc, hwnd, message, wp, lp)
}

func viewerControl(class, caption string, style, exStyle, parent, id, instance uintptr) uintptr {
	return call(createWindow, exStyle, uintptr(unsafe.Pointer(wide(class))),
		uintptr(unsafe.Pointer(wide(caption))), wsChild|wsVisible|style,
		0, 0, 1, 1, parent, id, instance, 0)
}

func layoutTrashViewer(hwnd uintptr) {
	if trashList == 0 {
		return
	}
	var area rect
	call(getClientRect, hwnd, uintptr(unsafe.Pointer(&area)))
	width, height := area.right, area.bottom
	if width < 1 || height < 1 {
		return
	}
	const margin, gap, buttonHeight = int32(14), int32(12), int32(28)
	contentWidth := width - 2*margin
	listWidth := contentWidth * 40 / 100
	previewWidth := contentWidth - listWidth - gap
	contentHeight := height - 2*margin - buttonHeight - gap
	if contentHeight < 1 {
		contentHeight = 1
	}
	if listWidth < 1 {
		listWidth = 1
	}
	if previewWidth < 1 {
		previewWidth = 1
	}
	previewX := margin + listWidth + gap
	call(moveWindow, trashList, uintptr(margin), uintptr(margin), uintptr(listWidth), uintptr(contentHeight), 1)
	call(moveWindow, trashPreview, uintptr(previewX), uintptr(margin), uintptr(previewWidth), uintptr(contentHeight), 1)
	buttonY := margin + contentHeight + gap
	call(moveWindow, trashDelete, uintptr(margin), uintptr(buttonY), 104, uintptr(buttonHeight), 1)
	call(moveWindow, trashEmpty, uintptr(margin+104+gap), uintptr(buttonY), 104, uintptr(buttonHeight), 1)
	restoreX := width - margin - 112
	closeX := restoreX - gap - 80
	if closeX < 0 {
		closeX = 0
	}
	call(moveWindow, trashClose, uintptr(closeX), uintptr(buttonY), 80, uintptr(buttonHeight), 1)
	if restoreX < 0 {
		restoreX = 0
	}
	call(moveWindow, trashRestore, uintptr(restoreX), uintptr(buttonY), 112, uintptr(buttonHeight), 1)
}

func trashLabel(state noteState) string {
	first := stdstrings.TrimSpace(stdstrings.SplitN(stdstrings.ReplaceAll(state.Text, "\r", ""), "\n", 2)[0])
	if first == "" {
		first = "(empty note)"
	}
	runes := []rune(first)
	if len(runes) > 50 {
		first = string(runes[:50]) + "…"
	}
	return first
}

func refreshTrashViewer() {
	if trashViewer == 0 || trashList == 0 {
		return
	}
	selected := selectedTrashIndex()
	selectedID := ""
	if selected >= 0 {
		selectedID = trash[selected].ID
	}
	call(sendMessage, trashList, lbResetContent, 0, 0)
	for i := len(trash) - 1; i >= 0; i-- {
		label := syscall.StringToUTF16Ptr(trashLabel(trash[i]))
		call(sendMessage, trashList, lbAddString, 0, uintptr(unsafe.Pointer(label)))
		runtime.KeepAlive(label)
	}
	row := 0
	for i := len(trash) - 1; i >= 0; i-- {
		if selectedID != "" && trash[i].ID == selectedID {
			row = len(trash) - 1 - i
			break
		}
	}
	if len(trash) > 0 {
		call(sendMessage, trashList, lbSetCurSel, uintptr(row), 0)
	}
	showSelectedTrashPreview()
}

func selectedTrashIndex() int {
	if trashList == 0 {
		return -1
	}
	row := call(sendMessage, trashList, lbGetCurSel, 0, 0)
	if row == ^uintptr(0) || int(row) >= len(trash) {
		return -1
	}
	return len(trash) - 1 - int(row)
}

func showSelectedTrashPreview() {
	if trashPreview == 0 {
		return
	}
	index := selectedTrashIndex()
	call(enableWindow, trashRestore, boolToUintptr(index >= 0))
	call(enableWindow, trashDelete, boolToUintptr(index >= 0))
	call(enableWindow, trashEmpty, boolToUintptr(len(trash) > 0))
	value := ""
	if index >= 0 {
		value = trash[index].Text
	}
	buffer := syscall.StringToUTF16Ptr(value)
	call(setWindowText, trashPreview, uintptr(unsafe.Pointer(buffer)))
	runtime.KeepAlive(buffer)
	call(sendMessage, trashPreview, emSetSel, 0, 0)
}

func deleteSelectedTrash() {
	index := selectedTrashIndex()
	if index < 0 {
		return
	}
	answer := call(messageBox, trashViewer,
		uintptr(unsafe.Pointer(wide("Permanently delete the selected note? This cannot be undone."))),
		uintptr(unsafe.Pointer(wide("Delete note"))), 0x134) // Yes/No, warning icon, No by default.
	if answer != 6 {
		return
	}
	updated := append(append([]noteState(nil), trash[:index]...), trash[index+1:]...)
	if err := saveTrash(dataDir, updated); err != nil {
		showSaveError(err)
		return
	}
	trash = updated
	refreshTrashViewer()
}

func emptyTrash() {
	if len(trash) == 0 {
		return
	}
	answer := call(messageBox, trashViewer,
		uintptr(unsafe.Pointer(wide("Permanently delete every note in Trash? This cannot be undone."))),
		uintptr(unsafe.Pointer(wide("Empty Trash"))), 0x134) // Yes/No, warning icon, No by default.
	if answer != 6 {
		return
	}
	if err := saveTrash(dataDir, nil); err != nil {
		showSaveError(err)
		return
	}
	trash = nil
	refreshTrashViewer()
}

func boolToUintptr(value bool) uintptr {
	if value {
		return 1
	}
	return 0
}
