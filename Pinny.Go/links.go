//go:build windows

package main

import (
	"net/url"
	"regexp"
	stdstrings "strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	getKeyState  = user32.NewProc("GetKeyState")
	getDC        = user32.NewProc("GetDC")
	releaseDC    = user32.NewProc("ReleaseDC")
	textExtent   = gdi32.NewProc("GetTextExtentPoint32W")
	shellExecute = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")
	webLinks     = regexp.MustCompile(`(?i)\b(?:https?://|www\.)[^\s<>"']+`)
)

type webLink struct {
	start, end int
	url        string
}

func findLinks(text string) []webLink {
	var links []webLink
	for _, match := range webLinks.FindAllStringIndex(text, -1) {
		candidate := stdstrings.TrimRight(text[match[0]:match[1]], ".,;:!?")
		for _, pair := range [][2]string{{"(", ")"}, {"[", "]"}, {"{", "}"}} {
			for stdstrings.HasSuffix(candidate, pair[1]) && stdstrings.Count(candidate, pair[1]) > stdstrings.Count(candidate, pair[0]) {
				candidate = stdstrings.TrimSuffix(candidate, pair[1])
			}
		}
		candidate = stdstrings.TrimRight(candidate, ".,;:!?")
		start := len(utf16.Encode([]rune(text[:match[0]])))
		end := start + len(utf16.Encode([]rune(candidate)))
		if stdstrings.HasPrefix(stdstrings.ToLower(candidate), "www.") {
			candidate = "https://" + candidate
		}
		parsed, err := url.Parse(candidate)
		if err == nil && parsed.Hostname() != "" && (stdstrings.EqualFold(parsed.Scheme, "http") || stdstrings.EqualFold(parsed.Scheme, "https")) {
			links = append(links, webLink{start, end, candidate})
		}
	}
	return links
}

func linkAt(text string, index int) string {
	for _, link := range findLinks(text) {
		if index >= link.start && index < link.end {
			return link.url
		}
	}
	return ""
}

func editorLinkAt(editor, position uintptr) string {
	var client rect
	call(getClientRect, editor, uintptr(unsafe.Pointer(&client)))
	pt := point{low(position), high(position)}
	if !containsPoint(client, pt) {
		return ""
	}
	dc := call(getDC, editor)
	if dc == 0 {
		return ""
	}
	defer call(releaseDC, editor, dc)
	text := editorText(editor)
	areas := visibleLinkAreas(editor, dc, text)
	// Prefer the actual link rectangle when two padded targets overlap.
	for _, area := range areas {
		if containsPoint(area.bounds, pt) {
			return area.url
		}
	}
	// Padding must not turn nearby ordinary text into a link target.
	if editorCharacterContains(editor, dc, text, position) {
		return ""
	}
	for _, area := range areas {
		bounds := area.bounds
		bounds.left -= 4
		bounds.right += 4
		bounds.top -= 3
		bounds.bottom += 3
		if containsPoint(bounds, pt) {
			return area.url
		}
	}
	return ""
}

func containsPoint(bounds rect, pt point) bool {
	return pt.x >= bounds.left && pt.x < bounds.right && pt.y >= bounds.top && pt.y < bounds.bottom
}

func editorCharacterContains(editor, dc uintptr, text string, position uintptr) bool {
	hit := call(sendMessage, editor, 0x00D7, 0, position) // EM_CHARFROMPOS
	// Recover the full UTF-16 index from the wrapped line, rather than using
	// only the truncated 16-bit character index returned by EM_CHARFROMPOS.
	firstVisible := call(sendMessage, editor, 0x00CE, 0, 0) // EM_GETFIRSTVISIBLELINE
	line := (hit >> 16) & 0xffff
	line += firstVisible &^ 0xffff
	if line < firstVisible {
		line += 0x10000
	}
	lineStart := call(sendMessage, editor, 0x00BB, line, 0) // EM_LINEINDEX
	if int32(lineStart) < 0 {
		return false
	}
	index := int(lineStart + ((hit - lineStart) & 0xffff))
	character := utf16.Encode([]rune(text))
	oldFont := call(selectObject, dc, call(sendMessage, editor, 0x0031, 0, 0)) // WM_GETFONT
	defer call(selectObject, dc, oldFont)
	// The nearest caret may be after the glyph under the pointer.
	for _, candidate := range []int{index, index - 1} {
		if candidate < 0 || candidate >= len(character) {
			continue
		}
		if character[candidate] >= 0xdc00 && character[candidate] <= 0xdfff && candidate > 0 {
			candidate--
		}
		if character[candidate] <= ' ' {
			continue
		}
		charPosition := call(sendMessage, editor, 0x00D6, uintptr(candidate), 0)
		count := 1
		if character[candidate] >= 0xd800 && character[candidate] <= 0xdbff && candidate+1 < len(character) {
			count = 2
		}
		var size point
		if call(textExtent, dc, uintptr(unsafe.Pointer(&character[candidate])), uintptr(count), uintptr(unsafe.Pointer(&size))) == 0 {
			continue
		}
		left, top := low(charPosition), high(charPosition)
		if containsPoint(rect{left, top, left + size.x, top + size.y}, point{low(position), high(position)}) {
			return true
		}
	}
	return false
}

type linkArea struct {
	bounds rect
	url    string
}

// Group glyphs into one continuous rectangle per wrapped line. Hit testing
// and underline painting use the same geometry, including scrolled notes.
func visibleLinkAreas(editor, dc uintptr, text string) []linkArea {
	oldFont := call(selectObject, dc, call(sendMessage, editor, 0x0031, 0, 0))
	defer call(selectObject, dc, oldFont)
	var client rect
	call(getClientRect, editor, uintptr(unsafe.Pointer(&client)))
	var height point
	if call(textExtent, dc, uintptr(unsafe.Pointer(wide("M"))), 1, uintptr(unsafe.Pointer(&height))) == 0 || height.y < 1 {
		return nil
	}
	firstLine := call(sendMessage, editor, 0x00CE, 0, 0)
	first := int(int32(call(sendMessage, editor, 0x00BB, firstLine, 0)))
	if first < 0 {
		return nil
	}
	characters := utf16.Encode([]rune(text))
	last := int(int32(call(sendMessage, editor, 0x00BB, firstLine+uintptr(client.bottom/height.y)+2, 0)))
	if last < 0 || last > len(characters) {
		last = len(characters)
	}
	var areas []linkArea
	for _, link := range findLinks(text) {
		start, end := link.start, link.end
		if start < first {
			start = first
		}
		if end > last {
			end = last
		}
		var segment *linkArea
		for index := start; index < end; {
			position := call(sendMessage, editor, 0x00D6, uintptr(index), 0)
			x, y := low(position), high(position)
			count := 1
			if characters[index] >= 0xd800 && characters[index] <= 0xdbff && index+1 < end {
				count = 2
			}
			var size point
			call(textExtent, dc, uintptr(unsafe.Pointer(&characters[index])), uintptr(count), uintptr(unsafe.Pointer(&size)))
			if y+size.y > 0 && y < client.bottom {
				if segment == nil || segment.bounds.top != y {
					areas = append(areas, linkArea{rect{x, y, x + size.x, y + size.y}, link.url})
					segment = &areas[len(areas)-1]
				} else {
					if x < segment.bounds.left {
						segment.bounds.left = x
					}
					if x+size.x > segment.bounds.right {
						segment.bounds.right = x + size.x
					}
				}
			}
			index += count
		}
	}
	return areas
}
