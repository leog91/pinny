//go:build windows

package main

import (
	"runtime"
	"syscall"
	"testing"
	"unicode/utf16"
	"unsafe"
)

func TestPreviousWordStart(t *testing.T) {
	for _, tc := range []struct{ text, remaining string }{
		{"", ""}, {"hello world", "hello "}, {"hello world   ", "hello "},
		{"hello\r\nworld", "hello\r\n"}, {"hello\r\n", ""},
		{"hello,world", "hello,"}, {"hello!!!", "hello"},
		{"hello café", "hello "}, {"hello 日本語", "hello "},
		{"hello 😀", "hello "}, {"hello e\u0301", "hello "},
	} {
		text := utf16.Encode([]rune(tc.text))
		got := string(utf16.Decode(text[:previousWordStart(text, len(text))]))
		if got != tc.remaining {
			t.Errorf("delete word from %q: got %q, want %q", tc.text, got, tc.remaining)
		}
	}
}

func TestWordDeletionInNativeEditor(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	editor := call(createWindow, 0, uintptr(unsafe.Pointer(wide("EDIT"))), 0, wsPopup|esMultiline, 0, 0, 320, 320, 0, 0, 0, 0)
	if editor == 0 {
		t.Fatal("could not create test editor")
	}
	defer call(destroyWindow, editor)
	read := func() string {
		text := make([]uint16, int(call(getWindowTextLength, editor))+1)
		call(getWindowText, editor, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
		return syscall.UTF16ToString(text)
	}
	call(setWindowText, editor, uintptr(unsafe.Pointer(wide("hello world"))))
	call(sendMessage, editor, emSetSel, 11, 11)
	deletePreviousWord(editor)
	if got := read(); got != "hello " {
		t.Fatalf("word deletion: %q", got)
	}
	call(sendMessage, editor, 0x0304, 0, 0) // WM_UNDO
	if got := read(); got != "hello world" {
		t.Fatalf("undo: %q", got)
	}
	call(sendMessage, editor, emSetSel, 1, 4)
	deletePreviousWord(editor)
	if got := read(); got != "ho world" {
		t.Fatalf("selection deletion: %q", got)
	}
	call(sendMessage, editor, emSetSel, 0, 0)
	deletePreviousWord(editor)
	if got := read(); got != "ho world" {
		t.Fatalf("deletion at start: %q", got)
	}
}
