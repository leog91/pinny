//go:build windows

package main

import (
	stdstrings "strings"
	"syscall"
	"unicode"
	"unsafe"
)

func cleanTitle(title string) string {
	title = stdstrings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, title)
	runes := []rune(stdstrings.TrimSpace(title))
	if len(runes) > 100 {
		runes = runes[:100]
	}
	return string(runes)
}

func displayTitle(title string) string {
	if title == "" {
		return "Pinny"
	}
	return title
}

func editorText(editor uintptr) string {
	buffer := make([]uint16, int(call(getWindowTextLength, editor))+1)
	call(getWindowText, editor, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	return syscall.UTF16ToString(buffer)
}

func currentTitle(n *note) string {
	if n.renameActive && !n.renamingStack {
		return cleanTitle(editorText(n.titleEdit))
	}
	return n.title
}

func noteWindowTitle(n *note) string {
	if n.stack != nil {
		return n.stack.name
	}
	return displayTitle(n.title)
}

func updateWindowTitle(hwnd uintptr, n *note) {
	title := noteWindowTitle(n)
	if n.stack != nil || n.title != "" {
		title += " — Pinny"
	}
	call(setWindowText, hwnd, uintptr(unsafe.Pointer(wide(title))))
}

func beginRename(hwnd uintptr, n *note) {
	beginTitleEdit(n, n.title)
}

func beginStackRename(hwnd uintptr, n *note) {
	finishRename(hwnd, n, true)
	n.renamingStack = true
	beginTitleEdit(n, n.stack.name)
}

func beginTitleEdit(n *note, value string) {
	if n.renameActive {
		return
	}
	n.renameActive = true
	call(setWindowText, n.titleEdit, uintptr(unsafe.Pointer(wide(value))))
	call(showWindow, n.titleEdit, 5)
	call(setFocus, n.titleEdit)
	call(sendMessage, n.titleEdit, emSetSel, 0, ^uintptr(0))
}

func finishRename(hwnd uintptr, n *note, commit bool) {
	if !n.renameActive {
		return
	}
	if commit {
		title := cleanTitle(editorText(n.titleEdit))
		if n.renamingStack {
			if title != "" {
				n.stack.name = title
			}
		} else if title != n.title {
			n.title = title
			n.dates.markModified()
		}
	}
	n.renameActive = false
	n.renamingStack = false
	call(showWindow, n.titleEdit, 0)
	updateWindowTitle(hwnd, n)
	if n.stack != nil && commit {
		n.stack.notes[n.stack.active] = activeNoteState(n)
		refreshStackList(n)
	}
	call(invalidateRect, hwnd, 0, 0)
	// Also resave after cancellation in case another note's autosave included
	// this title while it was being edited.
	queueSave(hwnd)
}
