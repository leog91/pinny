//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const linkHintTimer = 2

type toolInfo struct {
	size, flags     uint32
	hwnd, id        uintptr
	area            rect
	instance        uintptr
	text            *uint16
	param, reserved uintptr
}

type linkHint struct {
	tooltip  uintptr
	tool     toolInfo
	link     string
	position point
	visible  bool
}

var (
	commonControls       = syscall.NewLazyDLL("comctl32.dll")
	initCommonControls   = commonControls.NewProc("InitCommonControlsEx")
	setWindowSubclass    = commonControls.NewProc("SetWindowSubclass")
	removeWindowSubclass = commonControls.NewProc("RemoveWindowSubclass")
	defSubclassProc      = commonControls.NewProc("DefSubclassProc")
	setCursor            = user32.NewProc("SetCursor")
	getCursorPos         = user32.NewProc("GetCursorPos")
	createPen            = gdi32.NewProc("CreatePen")
	moveTo               = gdi32.NewProc("MoveToEx")
	lineTo               = gdi32.NewProc("LineTo")
	saveDC               = gdi32.NewProc("SaveDC")
	restoreDC            = gdi32.NewProc("RestoreDC")
	intersectClipRect    = gdi32.NewProc("IntersectClipRect")
	linkCueCallback      uintptr
)

func installLinkCues(hwnd uintptr, n *note) error {
	if linkCueCallback == 0 {
		linkCueCallback = syscall.NewCallback(linkCueProc)
	}
	controls := struct{ size, classes uint32 }{8, 0xff}
	if call(initCommonControls, uintptr(unsafe.Pointer(&controls))) == 0 {
		return fmt.Errorf("could not initialize link tooltips")
	}
	hint := &n.linkHint
	hint.tooltip = call(createWindow, 8, uintptr(unsafe.Pointer(wide("tooltips_class32"))), 0, wsPopup|3, 0, 0, 0, 0, hwnd, 0, 0, 0)
	if hint.tooltip == 0 {
		return fmt.Errorf("could not create link tooltip")
	}
	// Use the v2 structure size: the app also supports common controls v5,
	// which rejects the newer trailing reserved field without a v6 manifest.
	hint.tool = toolInfo{size: uint32(unsafe.Offsetof(toolInfo{}.reserved)), flags: 0x20, hwnd: n.edit, id: 1, text: wide("Ctrl+click to open link")} // TTF_TRACK
	if call(sendMessage, hint.tooltip, 0x0432, 0, uintptr(unsafe.Pointer(&hint.tool))) == 0 {
		return fmt.Errorf("could not register link tooltip")
	}
	if call(setWindowSubclass, n.edit, linkCueCallback, 1, hwnd) == 0 {
		return fmt.Errorf("could not enable link styling")
	}
	return nil
}

func hideLinkHint(n *note) {
	call(killTimer, n.edit, linkHintTimer)
	hint := &n.linkHint
	if hint.visible {
		call(sendMessage, hint.tooltip, 0x0411, 0, uintptr(unsafe.Pointer(&hint.tool))) // TTM_TRACKACTIVATE
	}
	hint.visible = false
	hint.link = ""
}

func cursorPosition(editor uintptr) point {
	var pt point
	call(getCursorPos, uintptr(unsafe.Pointer(&pt)))
	call(screenToClient, editor, uintptr(unsafe.Pointer(&pt)))
	return pt
}

func packedPoint(pt point) uintptr {
	return uintptr(uint16(pt.x)) | uintptr(uint16(pt.y))<<16
}

func refreshLinkCursor(editor uintptr) {
	cursor := linkCursor(editor, cursorPosition(editor), call(getKeyState, 0x11)&0x8000 != 0)
	call(setCursor, call(loadCursor, 0, cursor))
}

func linkCursor(editor uintptr, position point, control bool) uintptr {
	if control && editorLinkAt(editor, packedPoint(position)) != "" {
		return 32649 // IDC_HAND
	}
	return 32513 // IDC_IBEAM
}

func showLinkHint(n *note, position point) {
	hint := &n.linkHint
	call(clientToScreen, n.edit, uintptr(unsafe.Pointer(&position)))
	position.x += 12
	position.y += 18
	call(sendMessage, hint.tooltip, 0x0412, 0, packedPoint(position))
	call(sendMessage, hint.tooltip, 0x0411, 1, uintptr(unsafe.Pointer(&hint.tool)))
	hint.visible = true
}

func linkCueProc(editor, message, wp, lp, id, owner uintptr) uintptr {
	n := windows[owner]
	if message == 0x0082 { // WM_NCDESTROY
		call(removeWindowSubclass, editor, linkCueCallback, id)
		return call(defSubclassProc, editor, message, wp, lp)
	}
	if n == nil {
		return call(defSubclassProc, editor, message, wp, lp)
	}
	switch message {
	case 0x0020: // WM_SETCURSOR
		if low(lp) == 1 {
			refreshLinkCursor(editor)
			return 1
		}
	case wmMouseMove:
		pt := point{low(lp), high(lp)}
		link := editorLinkAt(editor, lp)
		hint := &n.linkHint
		if link != hint.link || (link != "" && pt != hint.position) {
			hideLinkHint(n)
			hint.link, hint.position = link, pt
			if link != "" && wp&1 == 0 {
				call(setTimer, editor, linkHintTimer, 500, 0)
			}
		}
		event := trackMouseEvent{size: uint32(unsafe.Sizeof(trackMouseEvent{})), flags: 2, hwnd: editor}
		call(trackMouse, uintptr(unsafe.Pointer(&event)))
	case wmTimer:
		if wp == linkHintTimer {
			call(killTimer, editor, linkHintTimer)
			hint := &n.linkHint
			if hint.link != "" && editorLinkAt(editor, packedPoint(cursorPosition(editor))) == hint.link {
				showLinkHint(n, cursorPosition(editor))
			}
			return 0
		}
	case wmMouseLeave, wmLButtonDown, 0x0008, 0x0100, 0x020A, 0x0115: // blur, key, wheel, scroll
		hideLinkHint(n)
	}
	result := call(defSubclassProc, editor, message, wp, lp)
	switch message {
	case wmPaint:
		dc := call(getDC, editor)
		if dc != 0 {
			drawLinkUnderlines(editor, dc, palettes[n.theme].mutedColor)
			call(releaseDC, editor, dc)
		}
	case 0x0318: // WM_PRINTCLIENT
		drawLinkUnderlines(editor, wp, palettes[n.theme].mutedColor)
	case wmMouseMove:
		if wp&1 != 0 {
			call(invalidateRect, editor, 0, 0)
		}
	case 0x0100, wmChar, wmLButtonDown, wmLButtonUp, 0x020A, 0x0115, wmSize:
		// Native EDIT also paints synchronously during edits and scrolling.
		call(invalidateRect, editor, 0, 0)
	}
	return result
}

func drawLinkUnderlines(editor, dc uintptr, color uint32) {
	areas := visibleLinkAreas(editor, dc, editorText(editor))
	if len(areas) == 0 {
		return
	}
	state := call(saveDC, dc)
	defer call(restoreDC, dc, state)
	var area rect
	call(getClientRect, editor, uintptr(unsafe.Pointer(&area)))
	margins := call(sendMessage, editor, 0x00D4, 0, 0) // EM_GETMARGINS
	left, right := int32(margins&0xffff), area.right-int32((margins>>16)&0xffff)
	call(intersectClipRect, dc, uintptr(left), 0, uintptr(right), uintptr(area.bottom))
	call(selectObject, dc, call(sendMessage, editor, 0x0031, 0, 0))
	pen := call(createPen, 0, 1, uintptr(color))
	if pen == 0 {
		return
	}
	oldPen := call(selectObject, dc, pen)
	defer func() { call(selectObject, dc, oldPen); call(deleteObject, pen) }()
	for _, link := range areas {
		bounds := link.bounds
		call(moveTo, dc, uintptr(bounds.left), uintptr(bounds.bottom-1), 0)
		call(lineTo, dc, uintptr(bounds.right), uintptr(bounds.bottom-1))
	}
}
