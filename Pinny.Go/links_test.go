//go:build windows

package main

import (
	"runtime"
	stdstrings "strings"
	"testing"
	"unicode/utf16"
	"unsafe"
)

func TestLinkAt(t *testing.T) {
	for _, tc := range []struct{ text, clicked, want string }{
		{"See https://example.com/path?q=1#part", "example", "https://example.com/path?q=1#part"},
		{"See www.example.com.", "example", "https://www.example.com"},
		{"(https://example.com/path).", "example", "https://example.com/path"},
		{"https://example.com/a_(b)", "example", "https://example.com/a_(b)"},
		{"😀 café https://example.com", "example", "https://example.com"},
		{"HTTPS://example.com", "example", "HTTPS://example.com"},
		{"file://example.com", "example", ""},
		{"javascript:alert(1)", "alert", ""},
		{"https://", "https", ""},
		{"https://example.com.", ".", ""},
		{"https://example.com next", "next", ""},
	} {
		// Locate the requested character using UTF-16 offsets, as Windows does.
		at := -1
		for i := len(tc.text) - len(tc.clicked); i >= 0; i-- {
			if tc.text[i:i+len(tc.clicked)] == tc.clicked {
				at = len(utf16.Encode([]rune(tc.text[:i])))
				break
			}
		}
		if got := linkAt(tc.text, at); got != tc.want {
			t.Errorf("link in %q at %q: got %q, want %q", tc.text, tc.clicked, got, tc.want)
		}
	}
}

func TestNativeLinkHitTesting(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	parent := call(createWindow, 0, uintptr(unsafe.Pointer(wide("STATIC"))), 0, wsPopup, 0, 0, 320, 320, 0, 0, 0, 0)
	if parent == 0 {
		t.Fatal("could not create test window")
	}
	defer call(destroyWindow, parent)
	editor := call(createWindow, 0, uintptr(unsafe.Pointer(wide("EDIT"))), 0, wsChild|wsVisible|esMultiline|esAutoVScroll|wsVScroll, 0, 0, 320, 320, parent, 0, 0, 0)
	if editor == 0 {
		t.Fatal("could not create link test editor")
	}
	defer call(destroyWindow, editor)
	call(showWindow, parent, 4)
	for _, prefix := range []string{"", "😀 café ", stdstrings.Repeat("padding ", 10000)} {
		text := prefix + "https://example.com/long/path/that/wraps/across/the/editor next"
		call(setWindowText, editor, uintptr(unsafe.Pointer(wide(text))))
		index := len(utf16.Encode([]rune(prefix))) + 12
		call(sendMessage, editor, emSetSel, uintptr(index), uintptr(index))
		call(sendMessage, editor, 0x00B7, 0, 0)                      // EM_SCROLLCARET
		line := call(sendMessage, editor, 0x00C9, uintptr(index), 0) // EM_LINEFROMCHAR
		call(sendMessage, editor, 0x00B6, 0, line)                   // EM_LINESCROLL also works for hidden test controls
		pos := call(sendMessage, editor, 0x00D6, uintptr(index), 0)
		click := uintptr(uint16(low(pos)+2)) | uintptr(uint16(high(pos)+2))<<16
		if got := editorLinkAt(editor, click); got != "https://example.com/long/path/that/wraps/across/the/editor" {
			t.Fatalf("hit link after %d UTF-16 units: %q", index, got)
		}
		if linkCursor(editor, point{low(click), high(click)}, true) != 32649 || linkCursor(editor, point{low(click), high(click)}, false) != 32513 {
			t.Fatal("link cursor did not distinguish Ctrl from normal editing")
		}
	}
	call(setWindowText, editor, uintptr(unsafe.Pointer(wide("https://example.com"))))
	if got := editorLinkAt(editor, 300|(5<<16)); got != "" {
		t.Fatalf("empty space opened link: %q", got)
	}
	if linkCursor(editor, point{300, 5}, true) != 32513 {
		t.Fatal("Ctrl changed the cursor outside a link")
	}
	// Hover and clicking share the padded target, including short wrapped
	// continuation lines like the final "1" in a long link.
	for _, text := range []string{"before\r\nhttps://example.com\r\nafter", "before\r\nhttps://example.com/" + stdstrings.Repeat("a", 80) + "1\r\nafter"} {
		call(setWindowText, editor, uintptr(unsafe.Pointer(wide(text))))
		source := call(getDC, editor)
		areas := visibleLinkAreas(editor, source, text)
		call(releaseDC, editor, source)
		if len(areas) == 0 {
			t.Fatal("no visible link rectangles")
		}
		for _, area := range areas {
			pt := point{area.bounds.right + 2, (area.bounds.top + area.bounds.bottom) / 2}
			var client rect
			call(getClientRect, editor, uintptr(unsafe.Pointer(&client)))
			if !containsPoint(client, pt) {
				continue
			}
			if got := editorLinkAt(editor, packedPoint(pt)); got != area.url {
				t.Fatalf("padding outside wrapped link: got %q, want %q", got, area.url)
			}
			if linkCursor(editor, pt, true) != 32649 || linkCursor(editor, pt, false) != 32513 {
				t.Fatal("padded cursor target does not match clicking")
			}
			far := point{area.bounds.right + 8, pt.y}
			if far.x < 300 && editorLinkAt(editor, packedPoint(far)) != "" {
				t.Fatal("link padding extends too far")
			}
		}
		// The next line's ordinary text must remain an editing target.
		index := len(utf16.Encode([]rune(text))) - 3
		pos := call(sendMessage, editor, 0x00D6, uintptr(index), 0)
		if editorLinkAt(editor, packedPoint(point{low(pos) + 2, high(pos) + 1})) != "" {
			t.Fatal("link padding captured neighboring text")
		}
	}
	call(setWindowText, editor, uintptr(unsafe.Pointer(wide("https://example.com"))))
	// Verify the visible underline against an off-screen bitmap without changing
	// text, selection, or undo history in the native editor.
	source := call(getDC, editor)
	defer call(releaseDC, editor, source)
	dc := call(gdi32.NewProc("CreateCompatibleDC"), source)
	defer call(gdi32.NewProc("DeleteDC"), dc)
	bitmap := call(gdi32.NewProc("CreateCompatibleBitmap"), source, 320, 320)
	oldBitmap := call(selectObject, dc, bitmap)
	defer func() { call(selectObject, dc, oldBitmap); call(deleteObject, bitmap) }()
	brush := call(createBrush, uintptr(rgb(255, 255, 255)))
	defer call(deleteObject, brush)
	area := rect{0, 0, 320, 320}
	call(fillRect, dc, uintptr(unsafe.Pointer(&area)), brush)
	color := rgb(100, 110, 120)
	drawLinkUnderlines(editor, dc, color)
	position := call(sendMessage, editor, 0x00D6, 2, 0)
	var size point
	call(selectObject, dc, call(sendMessage, editor, 0x0031, 0, 0))
	call(textExtent, dc, uintptr(unsafe.Pointer(wide("t"))), 1, uintptr(unsafe.Pointer(&size)))
	if got := call(gdi32.NewProc("GetPixel"), dc, uintptr(low(position)+1), uintptr(high(position)+size.y-1)); got != uintptr(color) {
		t.Fatalf("link underline pixel: %x, want %x", got, color)
	}
	if editorText(editor) != "https://example.com" {
		t.Fatal("styling changed note text")
	}
	n := &note{edit: editor}
	if err := installLinkCues(parent, n); err != nil {
		t.Fatal(err)
	}
	defer call(destroyWindow, n.linkHint.tooltip)
	showLinkHint(n, point{10, 10})
	if call(user32.NewProc("IsWindowVisible"), n.linkHint.tooltip) == 0 {
		t.Fatal("link tooltip did not show")
	}
	hideLinkHint(n)
	if call(user32.NewProc("IsWindowVisible"), n.linkHint.tooltip) != 0 {
		t.Fatal("link tooltip did not hide")
	}
}
