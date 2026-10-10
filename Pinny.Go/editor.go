//go:build windows

package main

import (
	"unicode"
	"unicode/utf16"
	"unsafe"
)

const (
	wmChar       = 0x0102
	emGetSel     = 0x00B0
	emReplaceSel = 0x00C2
)

func handleEditorMessage(message msg) bool {
	// TranslateMessage produces DEL for Ctrl+Backspace. The standard EDIT
	// control does not implement word deletion for this character.
	if message.message != wmChar || message.wParam != 0x7f {
		return false
	}
	for _, n := range windows {
		if n.edit == message.hwnd {
			deletePreviousWord(n.edit)
			return true
		}
	}
	return false
}

func deletePreviousWord(editor uintptr) {
	var start, end uint32
	call(sendMessage, editor, emGetSel, uintptr(unsafe.Pointer(&start)), uintptr(unsafe.Pointer(&end)))
	if start == end {
		if start == 0 {
			return
		}
		length := int(call(getWindowTextLength, editor))
		text := make([]uint16, length+1)
		call(getWindowText, editor, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
		start = uint32(previousWordStart(text[:length], int(start)))
	}
	call(sendMessage, editor, emSetSel, uintptr(start), uintptr(end))
	// Keep native undo and EN_CHANGE notifications (including autosave).
	call(sendMessage, editor, emReplaceSel, 1, uintptr(unsafe.Pointer(wide(""))))
}

func previousWordStart(text []uint16, caret int) int {
	if caret > len(text) {
		caret = len(text)
	}
	previous := func(at int) (int, rune) {
		at--
		r := rune(text[at])
		if at > 0 && r >= 0xdc00 && r <= 0xdfff && text[at-1] >= 0xd800 && text[at-1] <= 0xdbff {
			at--
			r = utf16.DecodeRune(rune(text[at]), r)
		}
		return at, r
	}
	// Remove trailing whitespace, then the preceding word or punctuation run.
	for caret > 0 {
		at, r := previous(caret)
		if !unicode.IsSpace(r) {
			break
		}
		caret = at
	}
	word := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || r == '_'
	}
	if caret > 0 {
		at, r := previous(caret)
		category := word(r)
		caret = at
		for caret > 0 {
			at, r = previous(caret)
			if unicode.IsSpace(r) || word(r) != category {
				break
			}
			caret = at
		}
	}
	return caret
}
