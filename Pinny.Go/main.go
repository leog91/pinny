//go:build windows

package main

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"unsafe"
)

// The UI uses direct Win32 calls. Normal launches save notes as local JSON;
// --notes remains an in-memory synthetic mode for manual comparison.
const (
	windowClass = "PinnyNativeNoteGo"
	headerH     = 32
	resizeEdge  = 6
	editID      = 100
	newID       = 1
	pinID       = 2
	trashID     = 3
	quitID      = 4
	lightID     = 5
	darkID      = 6
	paperID     = 7
	restoreID   = 8
	viewTrashID = 10
	exportID    = 11
	importID    = 12
	locationID  = 13

	wmDestroy       = 0x0002
	wmMove          = 0x0003
	wmSize          = 0x0005
	wmSetFocus      = 0x0007
	wmClose         = 0x0010
	wmPaint         = 0x000F
	wmEraseBkgnd    = 0x0014
	wmSetFont       = 0x0030
	wmNcCalcSize    = 0x0083
	wmNcHitTest     = 0x0084
	wmNcLButtonDown = 0x00A1
	wmCommand       = 0x0111
	wmTimer         = 0x0113
	wmCtlColorEdit  = 0x0133
	wmMouseMove     = 0x0200
	wmLButtonDown   = 0x0201
	wmLButtonUp     = 0x0202
	wmMouseLeave    = 0x02A3
	emLimitText     = 0x00C5
	emGetLineCount  = 0x00BA
	emSetMargins    = 0x00D3

	wsPopup       = 0x80000000
	wsThickFrame  = 0x00040000
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000
	wsChild       = 0x40000000
	wsVisible     = 0x10000000
	wsVScroll     = 0x00200000
	wsExAppWindow = 0x00040000
	esMultiline   = 0x0004
	esAutoVScroll = 0x0040
	esWantReturn  = 0x1000

	mfSeparator   = 0x0800
	tpmReturnCmd  = 0x0100
	dtCenter      = 0x0001
	dtVCenter     = 0x0004
	dtSingleLine  = 0x0020
	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoActivate = 0x0010
)

type point struct{ x, y int32 }
type rect struct{ left, top, right, bottom int32 }
type msg struct {
	hwnd           uintptr
	message        uint32
	_              uint32
	wParam, lParam uintptr
	time           uint32
	pt             point
	lPrivate       uint32
}
type wndClassEx struct {
	size, style                        uint32
	wndProc                            uintptr
	clsExtra, wndExtra                 int32
	instance, icon, cursor, background uintptr
	menuName, className                *uint16
	iconSmall                          uintptr
}
type paintStruct struct {
	dc                 uintptr
	erase              int32
	paint              rect
	restore, incUpdate int32
	reserved           [32]byte
}
type trackMouseEvent struct {
	size, flags uint32
	hwnd        uintptr
	hoverTime   uint32
}
type palette struct {
	surface, header, border, hover     uintptr
	surfaceColor, inkColor, mutedColor uint32
}
type note struct {
	edit, menu       uintptr
	id               string
	theme            string
	pinned           bool
	loading          bool
	hovered, pressed int
	scrollbar        bool
}
type initialNote struct {
	x, y, width, height int32
	id                  string
	text, theme         string
	pinned              bool
}

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	gdi32               = syscall.NewLazyDLL("gdi32.dll")
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	registerClass       = user32.NewProc("RegisterClassExW")
	createWindow        = user32.NewProc("CreateWindowExW")
	defWindowProc       = user32.NewProc("DefWindowProcW")
	destroyWindow       = user32.NewProc("DestroyWindow")
	showWindow          = user32.NewProc("ShowWindow")
	updateWindow        = user32.NewProc("UpdateWindow")
	getMessage          = user32.NewProc("GetMessageW")
	translateMessage    = user32.NewProc("TranslateMessage")
	dispatchMessage     = user32.NewProc("DispatchMessageW")
	postQuitMessage     = user32.NewProc("PostQuitMessage")
	getClientRect       = user32.NewProc("GetClientRect")
	getWindowRect       = user32.NewProc("GetWindowRect")
	getWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	getWindowText       = user32.NewProc("GetWindowTextW")
	screenToClient      = user32.NewProc("ScreenToClient")
	clientToScreen      = user32.NewProc("ClientToScreen")
	moveWindow          = user32.NewProc("MoveWindow")
	setWindowPos        = user32.NewProc("SetWindowPos")
	setFocus            = user32.NewProc("SetFocus")
	sendMessage         = user32.NewProc("SendMessageW")
	invalidateRect      = user32.NewProc("InvalidateRect")
	beginPaint          = user32.NewProc("BeginPaint")
	endPaint            = user32.NewProc("EndPaint")
	fillRect            = user32.NewProc("FillRect")
	frameRect           = user32.NewProc("FrameRect")
	drawText            = user32.NewProc("DrawTextW")
	createPopupMenu     = user32.NewProc("CreatePopupMenu")
	appendMenu          = user32.NewProc("AppendMenuW")
	destroyMenu         = user32.NewProc("DestroyMenu")
	trackPopupMenu      = user32.NewProc("TrackPopupMenu")
	trackMouse          = user32.NewProc("TrackMouseEvent")
	setForeground       = user32.NewProc("SetForegroundWindow")
	setCapture          = user32.NewProc("SetCapture")
	releaseCapture      = user32.NewProc("ReleaseCapture")
	showScrollBar       = user32.NewProc("ShowScrollBar")
	setTimer            = user32.NewProc("SetTimer")
	killTimer           = user32.NewProc("KillTimer")
	getSystemMetrics    = user32.NewProc("GetSystemMetrics")
	loadCursor          = user32.NewProc("LoadCursorW")
	loadImage           = user32.NewProc("LoadImageW")
	destroyIcon         = user32.NewProc("DestroyIcon")
	messageBox          = user32.NewProc("MessageBoxW")
	getModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	createBrush         = gdi32.NewProc("CreateSolidBrush")
	createFont          = gdi32.NewProc("CreateFontW")
	deleteObject        = gdi32.NewProc("DeleteObject")
	selectObject        = gdi32.NewProc("SelectObject")
	setBkMode           = gdi32.NewProc("SetBkMode")
	setTextColor        = gdi32.NewProc("SetTextColor")
	setBkColor          = gdi32.NewProc("SetBkColor")

	windows     = make(map[uintptr]*note)
	windowOrder []uintptr
	trash       []noteState
	palettes    = make(map[string]palette)
	// Proc.Call takes uintptr arguments, so keep all UTF-16 buffers rooted.
	strings            = make(map[string]*uint16)
	font               uintptr
	dataDir            string
	dataDirOverridden  bool
	persistenceEnabled bool
	saveTimerWindow    uintptr
	quitting           bool
	saveErrorShown     bool
)

//go:uintptrescapes
func call(p *syscall.LazyProc, args ...uintptr) uintptr {
	r, _, _ := p.Call(args...)
	return r
}
func wide(s string) *uint16 {
	if p := strings[s]; p != nil {
		return p
	}
	p := syscall.StringToUTF16Ptr(s)
	strings[s] = p
	return p
}
func rgb(r, g, b uint32) uint32 { return r | g<<8 | b<<16 }
func makePalette(sr, sg, sb, hr, hg, hb, br, bg, bb, vr, vg, vb, ir, ig, ib, mr, mg, mb uint32) palette {
	surface, header, border, hover := rgb(sr, sg, sb), rgb(hr, hg, hb), rgb(br, bg, bb), rgb(vr, vg, vb)
	return palette{
		surface: call(createBrush, uintptr(surface)), header: call(createBrush, uintptr(header)),
		border: call(createBrush, uintptr(border)), hover: call(createBrush, uintptr(hover)),
		surfaceColor: surface, inkColor: rgb(ir, ig, ib), mutedColor: rgb(mr, mg, mb),
	}
}
func low(v uintptr) int32  { return int32(int16(uint16(v))) }
func high(v uintptr) int32 { return int32(int16(uint16(v >> 16))) }
func clamp(v, min, max int32) int32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
func topmost(pinned bool) uintptr {
	if pinned {
		return ^uintptr(0)
	} // HWND_TOPMOST
	return ^uintptr(1) // HWND_NOTOPMOST
}

func main() {
	runtime.LockOSThread()
	if err := comError(call(coInitialize, 0, 2)); err != nil {
		showOperationError(0, "Initialize Windows dialogs", err)
		return
	}
	defer call(coUninitialize)
	count := 0
	switch {
	case len(os.Args) == 1:
		persistenceEnabled = true
		dataDir = os.Getenv("PINNY_DATA_DIR")
	case len(os.Args) == 3 && os.Args[1] == "--data-dir" && os.Args[2] != "":
		persistenceEnabled = true
		dataDir = os.Args[2]
	case len(os.Args) == 3 && os.Args[1] == "--notes":
		n, err := strconv.Atoi(os.Args[2])
		if err != nil || n < 1 || n > 100 {
			usage()
			return
		}
		count = n
	default:
		usage()
		return
	}
	if persistenceEnabled && dataDir == "" {
		var err error
		dataDir, err = savedDataDir()
		if err != nil {
			call(messageBox, 0, uintptr(unsafe.Pointer(wide(err.Error()))), uintptr(unsafe.Pointer(wide("Pinny"))), 0x10)
			return
		}
	} else if persistenceEnabled {
		dataDirOverridden = true
	}
	if err := run(count); err != nil {
		call(messageBox, 0, uintptr(unsafe.Pointer(wide(err.Error()))), uintptr(unsafe.Pointer(wide("Pinny"))), 0x10)
		os.Exit(1)
	}
}
func usage() {
	call(messageBox, 0, uintptr(unsafe.Pointer(wide("Usage: Pinny.exe [--data-dir FOLDER | --notes COUNT]"))),
		uintptr(unsafe.Pointer(wide("Pinny"))), 0x10)
}
func run(count int) error {
	var saved []noteState
	if persistenceEnabled {
		var err error
		saved, err = loadNotes(dataDir)
		if err != nil {
			return fmt.Errorf("could not load notes; saved data was not changed: %w", err)
		}
		trash, err = loadTrash(dataDir)
		if err != nil {
			return fmt.Errorf("could not load Trash; saved data was not changed: %w", err)
		}
	}
	className := wide(windowClass)
	instance := call(getModuleHandle, 0)
	// Resource group #1 is added to the release executable by Build-Native.ps1.
	largeIcon := call(loadImage, instance, 1, 1, 32, 32, 0)
	smallIcon := call(loadImage, instance, 1, 1, 16, 16, 0)
	defer func() {
		if largeIcon != 0 {
			call(destroyIcon, largeIcon)
		}
		if smallIcon != 0 {
			call(destroyIcon, smallIcon)
		}
	}()
	wc := wndClassEx{
		size:      uint32(unsafe.Sizeof(wndClassEx{})),
		wndProc:   syscall.NewCallback(wndProc),
		instance:  instance,
		icon:      largeIcon,
		cursor:    call(loadCursor, 0, 32512),
		className: className,
		iconSmall: smallIcon,
	}
	if call(registerClass, uintptr(unsafe.Pointer(&wc))) == 0 {
		return fmt.Errorf("could not register Win32 class")
	}
	if err := registerTrashViewerClass(instance, largeIcon, smallIcon); err != nil {
		return err
	}
	palettes["Light"] = makePalette(255, 255, 255, 244, 245, 247, 200, 205, 211, 225, 229, 235, 32, 35, 40, 85, 91, 99)
	palettes["Dark"] = makePalette(36, 39, 44, 48, 52, 59, 76, 84, 94, 67, 73, 82, 232, 234, 237, 189, 197, 205)
	palettes["Paper"] = makePalette(255, 248, 230, 235, 218, 183, 198, 177, 133, 222, 201, 161, 62, 51, 39, 107, 88, 59)
	font = call(createFont, uintptr(uint32(0xfffffff0)), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(wide("Segoe UI"))))
	defer func() {
		for _, p := range palettes {
			call(deleteObject, p.surface)
			call(deleteObject, p.header)
			call(deleteObject, p.border)
			call(deleteObject, p.hover)
		}
		if font != 0 {
			call(deleteObject, font)
		}
	}()

	if len(saved) != 0 {
		for _, state := range saved {
			if err := showSavedNote(state); err != nil {
				return err
			}
		}
	} else if count == 0 {
		if err := showNote(initialNote{width: 320, height: 320, theme: "Light", x: -2147483648, y: -2147483648}); err != nil {
			return err
		}
		queueSave(windowOrder[0])
	} else {
		for i := 0; i < count; i++ {
			offset := int32((i % 5) * 32)
			if err := showNote(initialNote{
				x: 80 + offset, y: 80 + offset, width: 320, height: 320, theme: "Light",
				text: fmt.Sprintf("Benchmark note %d\r\nA short plain-text note for comparison.", i),
			}); err != nil {
				return err
			}
		}
	}
	var message msg
	for int32(call(getMessage, uintptr(unsafe.Pointer(&message)), 0, 0, 0)) > 0 {
		call(translateMessage, uintptr(unsafe.Pointer(&message)))
		call(dispatchMessage, uintptr(unsafe.Pointer(&message)))
	}
	return nil
}
func showSavedNote(state noteState) error {
	theme := state.Theme
	if theme != "Dark" && theme != "Paper" {
		theme = "Light"
	}
	width, height := int32(state.Width), int32(state.Height)
	if math.IsNaN(state.Width) || math.IsInf(state.Width, 0) || width == 0 {
		width = 320
	}
	if math.IsNaN(state.Height) || math.IsInf(state.Height, 0) || height == 0 {
		height = 320
	}
	x, y := int32(state.Left), int32(state.Top)
	if math.IsNaN(state.Left) || math.IsInf(state.Left, 0) {
		x = -2147483648
	}
	if math.IsNaN(state.Top) || math.IsInf(state.Top, 0) {
		y = -2147483648
	}
	return showNote(initialNote{
		id: state.ID, x: x, y: y, width: width, height: height,
		text: state.Text, theme: theme, pinned: state.IsPinned,
	})
}
func showNote(input initialNote) error {
	if input.id == "" {
		var err error
		input.id, err = newNoteID()
		if err != nil {
			return fmt.Errorf("could not create note ID: %w", err)
		}
	}
	width, height := clamp(input.width, 210, 10000), clamp(input.height, 120, 10000)
	sx := int32(call(getSystemMetrics, 76))
	sy := int32(call(getSystemMetrics, 77))
	sw := int32(call(getSystemMetrics, 78))
	sh := int32(call(getSystemMetrics, 79))
	x, y := input.x, input.y
	if x == -2147483648 {
		x = sx + (sw-width)/2
	}
	if y == -2147483648 {
		y = sy + (sh-height)/2
	}
	x = clamp(x, sx-width+48, sx+sw-48)
	y = clamp(y, sy, sy+sh-48)
	hwnd := call(createWindow, wsExAppWindow, uintptr(unsafe.Pointer(wide(windowClass))),
		uintptr(unsafe.Pointer(wide("Pinny"))), wsPopup|wsThickFrame|wsSysMenu|wsMinimizeBox,
		uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0, 0, call(getModuleHandle, 0), 0)
	if hwnd == 0 {
		return fmt.Errorf("could not create note window")
	}
	n := &note{id: input.id, theme: input.theme, pinned: input.pinned, loading: true}
	windows[hwnd] = n
	windowOrder = append(windowOrder, hwnd)
	n.menu = call(createPopupMenu)
	call(appendMenu, n.menu, 0, lightID, uintptr(unsafe.Pointer(wide("Light"))))
	call(appendMenu, n.menu, 0, darkID, uintptr(unsafe.Pointer(wide("Dark"))))
	call(appendMenu, n.menu, 0, paperID, uintptr(unsafe.Pointer(wide("Paper"))))
	call(appendMenu, n.menu, mfSeparator, 0, 0)
	call(appendMenu, n.menu, 0, trashID, uintptr(unsafe.Pointer(wide("Move this note to Trash"))))
	call(appendMenu, n.menu, 0, viewTrashID, uintptr(unsafe.Pointer(wide("View Trash..."))))
	call(appendMenu, n.menu, 0, restoreID, uintptr(unsafe.Pointer(wide("Restore last deleted note"))))
	call(appendMenu, n.menu, mfSeparator, 0, 0)
	call(appendMenu, n.menu, 0, exportID, uintptr(unsafe.Pointer(wide("Export notes..."))))
	call(appendMenu, n.menu, 0, importID, uintptr(unsafe.Pointer(wide("Import notes..."))))
	call(appendMenu, n.menu, 0, locationID, uintptr(unsafe.Pointer(wide("Choose notes folder..."))))
	call(appendMenu, n.menu, mfSeparator, 0, 0)
	call(appendMenu, n.menu, 0, quitID, uintptr(unsafe.Pointer(wide("Quit Pinny (keep notes)"))))
	initialText := syscall.StringToUTF16(input.text)
	n.edit = call(createWindow, 0, uintptr(unsafe.Pointer(wide("EDIT"))),
		uintptr(unsafe.Pointer(&initialText[0])), wsChild|wsVisible|esMultiline|esAutoVScroll|esWantReturn|wsVScroll,
		1, headerH, uintptr(width-2), uintptr(height-headerH-1), hwnd, editID, 0, 0)
	runtime.KeepAlive(initialText)
	if n.edit == 0 {
		call(destroyWindow, hwnd)
		return fmt.Errorf("could not create note editor")
	}
	call(sendMessage, n.edit, emLimitText, 0x7fffffff, 0)
	call(sendMessage, n.edit, emSetMargins, 3, 10|(10<<16))
	call(sendMessage, n.edit, wmSetFont, font, 1)
	resizeEdit(hwnd, n)
	call(showScrollBar, n.edit, 1, 0)
	updateScrollbar(n)
	call(showWindow, hwnd, 5)
	if n.pinned {
		call(setWindowPos, hwnd, topmost(true), 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	}
	call(updateWindow, hwnd)
	call(setFocus, n.edit)
	n.loading = false
	return nil
}
func snapshotNotes(exclude uintptr) ([]noteState, error) {
	notes := make([]noteState, 0, len(windows))
	for _, hwnd := range windowOrder {
		if hwnd == exclude {
			continue
		}
		n := windows[hwnd]
		if n == nil || n.edit == 0 {
			continue
		}
		var bounds rect
		if call(getWindowRect, hwnd, uintptr(unsafe.Pointer(&bounds))) == 0 {
			return nil, fmt.Errorf("could not read note position")
		}
		length := int(call(getWindowTextLength, n.edit))
		buffer := make([]uint16, length+1)
		call(getWindowText, n.edit, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
		notes = append(notes, noteState{
			ID: n.id, Text: syscall.UTF16ToString(buffer),
			Left: float64(bounds.left), Top: float64(bounds.top),
			Width: float64(bounds.right - bounds.left), Height: float64(bounds.bottom - bounds.top),
			IsPinned: n.pinned, Theme: n.theme,
		})
	}
	return notes, nil
}
func saveAllExcept(exclude uintptr) error {
	if !persistenceEnabled {
		return nil
	}
	notes, err := snapshotNotes(exclude)
	if err != nil {
		return err
	}
	if err := saveNotes(dataDir, notes); err != nil {
		return err
	}
	saveErrorShown = false
	return nil
}
func showSaveError(err error) {
	if saveErrorShown {
		return
	}
	saveErrorShown = true
	message := "Pinny could not complete the save. Check notes.json and trash.json before retrying.\n\n" + err.Error()
	call(messageBox, 0, uintptr(unsafe.Pointer(wide(message))), uintptr(unsafe.Pointer(wide("Pinny"))), 0x10)
}
func stopSaveTimer() {
	if saveTimerWindow != 0 {
		call(killTimer, saveTimerWindow, 1)
		saveTimerWindow = 0
	}
}
func queueSave(hwnd uintptr) {
	if !persistenceEnabled || quitting {
		return
	}
	if n := windows[hwnd]; n == nil || n.loading {
		return
	}
	stopSaveTimer()
	if call(setTimer, hwnd, 1, 500, 0) == 0 {
		if err := saveAllExcept(0); err != nil {
			showSaveError(err)
		}
		return
	}
	saveTimerWindow = hwnd
}
func resizeEdit(hwnd uintptr, n *note) {
	if n.edit == 0 {
		return
	}
	var client rect
	call(getClientRect, hwnd, uintptr(unsafe.Pointer(&client)))
	width, height := client.right-2, client.bottom-headerH-10
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	call(moveWindow, n.edit, 1, headerH+2, uintptr(width), uintptr(height), 1)
	updateScrollbar(n)
}
func updateScrollbar(n *note) {
	if n.edit == 0 {
		return
	}
	var area rect
	call(getClientRect, n.edit, uintptr(unsafe.Pointer(&area)))
	lines := int32(call(sendMessage, n.edit, emGetLineCount, 0, 0))
	show := lines*16 > area.bottom
	if show != n.scrollbar {
		if show {
			call(showScrollBar, n.edit, 1, 1)
		} else {
			call(showScrollBar, n.edit, 1, 0)
		}
		n.scrollbar = show
	}
}
func buttonAt(hwnd, lp uintptr) int {
	x, y := low(lp), high(lp)
	if y < 0 || y >= headerH {
		return 0
	}
	var area rect
	call(getClientRect, hwnd, uintptr(unsafe.Pointer(&area)))
	if x < 0 || x >= area.right {
		return 0
	}
	switch {
	case x >= area.right-36:
		return 4
	case x >= area.right-72:
		return 3
	case x >= area.right-108:
		return 2
	case x >= area.right-144:
		return 1
	default:
		return 0
	}
}
func wndProc(hwnd, message, wp, lp uintptr) uintptr {
	if message == wmNcCalcSize {
		return 0
	}
	n := windows[hwnd]
	if n == nil {
		return call(defWindowProc, hwnd, message, wp, lp)
	}
	switch message {
	case wmNcHitTest:
		pt := point{low(lp), high(lp)}
		call(screenToClient, hwnd, uintptr(unsafe.Pointer(&pt)))
		var area rect
		call(getClientRect, hwnd, uintptr(unsafe.Pointer(&area)))
		ex, ey := 0, 0
		if pt.x < resizeEdge {
			ex = -1
		} else if pt.x >= area.right-resizeEdge {
			ex = 1
		}
		if pt.y < resizeEdge {
			ey = -1
		} else if pt.y >= area.bottom-resizeEdge {
			ey = 1
		}
		switch {
		case ex == -1 && ey == -1:
			return 13
		case ex == 1 && ey == -1:
			return 14
		case ex == -1 && ey == 1:
			return 16
		case ex == 1 && ey == 1:
			return 17
		case ex == -1:
			return 10
		case ex == 1:
			return 11
		case ey == -1:
			return 12
		case ey == 1:
			return 15
		default:
			return 1
		}
	case wmEraseBkgnd:
		return 1
	case wmPaint:
		paintHeader(hwnd, n)
		return 0
	case wmMouseMove:
		hover := buttonAt(hwnd, lp)
		if hover != n.hovered {
			n.hovered = hover
			call(invalidateRect, hwnd, 0, 0)
		}
		event := trackMouseEvent{size: uint32(unsafe.Sizeof(trackMouseEvent{})), flags: 2, hwnd: hwnd}
		call(trackMouse, uintptr(unsafe.Pointer(&event)))
		return 0
	case wmMouseLeave:
		n.hovered = 0
		call(invalidateRect, hwnd, 0, 0)
		return 0
	case wmLButtonDown:
		n.pressed = buttonAt(hwnd, lp)
		if n.pressed != 0 {
			call(setCapture, hwnd)
		} else if high(lp) < headerH {
			call(releaseCapture)
			call(sendMessage, hwnd, wmNcLButtonDown, 2, 0)
		}
		return 0
	case wmLButtonUp:
		call(releaseCapture)
		pressed := n.pressed
		n.pressed = 0
		if pressed != 0 && pressed == buttonAt(hwnd, lp) {
			clickButton(hwnd, n, pressed)
		}
		return 0
	case wmSize:
		resizeEdit(hwnd, n)
		queueSave(hwnd)
		return 0
	case wmMove:
		queueSave(hwnd)
		return 0
	case wmSetFocus:
		if n.edit != 0 {
			call(setFocus, n.edit)
		}
		return 0
	case wmCommand:
		id, notification := wp&0xffff, (wp>>16)&0xffff
		switch {
		case id == editID && notification == 0x300:
			updateScrollbar(n)
			queueSave(hwnd)
		case id == newID:
			newNote(hwnd, n)
		case id == pinID:
			togglePin(hwnd, n)
		case id == trashID:
			moveToTrash(hwnd)
		case id == restoreID:
			restoreLastDeleted()
		case id == viewTrashID:
			showTrashViewer()
		case id == quitID:
			quit()
		case id == exportID:
			exportNotes(hwnd)
		case id == importID:
			importNotes(hwnd)
		case id == locationID:
			changeNotesFolder(hwnd)
		case id == lightID || id == darkID || id == paperID:
			n.theme = "Light"
			if id == darkID {
				n.theme = "Dark"
			} else if id == paperID {
				n.theme = "Paper"
			}
			call(invalidateRect, n.edit, 0, 1)
			call(invalidateRect, hwnd, 0, 0)
			queueSave(hwnd)
		}
		return 0
	case wmTimer:
		if wp == 1 && hwnd == saveTimerWindow {
			stopSaveTimer()
			if err := saveAllExcept(0); err != nil {
				showSaveError(err)
			}
			return 0
		}
	case wmCtlColorEdit:
		p := palettes[n.theme]
		call(setTextColor, wp, uintptr(p.inkColor))
		call(setBkColor, wp, uintptr(p.surfaceColor))
		return p.surface
	case wmClose:
		quit()
		return 0
	case wmDestroy:
		if saveTimerWindow == hwnd {
			stopSaveTimer()
		}
		call(destroyMenu, n.menu)
		delete(windows, hwnd)
		for i, handle := range windowOrder {
			if handle == hwnd {
				windowOrder = append(windowOrder[:i], windowOrder[i+1:]...)
				break
			}
		}
		if len(windows) == 0 {
			call(postQuitMessage, 0)
		}
		return 0
	}
	return call(defWindowProc, hwnd, message, wp, lp)
}
func newNote(hwnd uintptr, n *note) {
	var bounds rect
	call(getWindowRect, hwnd, uintptr(unsafe.Pointer(&bounds)))
	if err := showNote(initialNote{
		x: bounds.left + 32, y: bounds.top + 32, width: bounds.right - bounds.left,
		height: bounds.bottom - bounds.top, theme: n.theme, pinned: n.pinned,
	}); err != nil {
		showSaveError(err)
		return
	}
	queueSave(windowOrder[len(windowOrder)-1])
}
func togglePin(hwnd uintptr, n *note) {
	n.pinned = !n.pinned
	call(setWindowPos, hwnd, topmost(n.pinned), 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	call(invalidateRect, hwnd, 0, 0)
	queueSave(hwnd)
}
func showInfo(hwnd uintptr, message string) {
	call(messageBox, hwnd, uintptr(unsafe.Pointer(wide(message))), uintptr(unsafe.Pointer(wide("Pinny"))), 0x40)
}
func moveToTrash(hwnd uintptr) {
	if !persistenceEnabled {
		showInfo(hwnd, "Trash is unavailable in --notes mode.")
		return
	}
	n := windows[hwnd]
	if n == nil {
		return
	}
	states, err := snapshotNotes(0)
	if err != nil {
		showSaveError(err)
		return
	}
	var current noteState
	found := false
	for _, state := range states {
		if state.ID == n.id {
			current, found = state, true
			break
		}
	}
	if !found {
		showSaveError(fmt.Errorf("could not find note to move to Trash"))
		return
	}
	updated := make([]noteState, 0, len(trash)+1)
	for _, state := range trash {
		if state.ID != current.ID {
			updated = append(updated, state)
		}
	}
	updated = append(updated, current)
	if err := saveTrash(dataDir, updated); err != nil {
		showSaveError(err)
		return
	}
	trash = updated
	refreshTrashViewer()
	stopSaveTimer()
	if err := saveAllExcept(hwnd); err != nil {
		showSaveError(err)
		return
	}
	call(destroyWindow, hwnd)
}
func restoreLastDeleted() {
	if !persistenceEnabled {
		showInfo(0, "Trash is unavailable in --notes mode.")
		return
	}
	if len(trash) == 0 {
		showInfo(0, "Trash is empty.")
		return
	}
	restoreTrashAt(len(trash) - 1)
}
func restoreTrashAt(index int) {
	if index < 0 || index >= len(trash) {
		return
	}
	selected := trash[index]
	updated := append(append([]noteState(nil), trash[:index]...), trash[index+1:]...)
	for _, n := range windows {
		if n.id == selected.ID {
			if err := saveTrash(dataDir, updated); err != nil {
				showSaveError(err)
				return
			}
			trash = updated
			refreshTrashViewer()
			showInfo(0, "This note is already open. Its duplicate Trash entry was removed.")
			return
		}
	}
	if err := showSavedNote(selected); err != nil {
		showSaveError(err)
		return
	}
	restoredWindow := windowOrder[len(windowOrder)-1]
	stopSaveTimer()
	if err := saveAllExcept(0); err != nil {
		call(destroyWindow, restoredWindow)
		showSaveError(err)
		return
	}
	if err := saveTrash(dataDir, updated); err != nil {
		showSaveError(err)
		return
	}
	trash = updated
	refreshTrashViewer()
}
func clickButton(hwnd uintptr, n *note, button int) {
	switch button {
	case 1:
		newNote(hwnd, n)
	case 2:
		togglePin(hwnd, n)
	case 3:
		var area rect
		call(getClientRect, hwnd, uintptr(unsafe.Pointer(&area)))
		pt := point{area.right - 72, headerH}
		call(clientToScreen, hwnd, uintptr(unsafe.Pointer(&pt)))
		call(setForeground, hwnd)
		choice := call(trackPopupMenu, n.menu, tpmReturnCmd, uintptr(pt.x), uintptr(pt.y), 0, hwnd, 0)
		if choice != 0 {
			call(sendMessage, hwnd, wmCommand, choice, 0)
		}
	case 4:
		quit()
	}
}
func quit() {
	stopSaveTimer()
	if err := saveAllExcept(0); err != nil {
		showSaveError(err)
		return
	}
	quitting = true
	handles := make([]uintptr, 0, len(windows))
	handles = append(handles, windowOrder...)
	for _, hwnd := range handles {
		call(destroyWindow, hwnd)
	}
}
func text(dc uintptr, value string, bounds rect, format uintptr, color uint32) {
	call(setTextColor, dc, uintptr(color))
	utf16 := syscall.StringToUTF16(value)
	call(drawText, dc, uintptr(unsafe.Pointer(&utf16[0])), uintptr(len(utf16)-1),
		uintptr(unsafe.Pointer(&bounds)), format)
	runtime.KeepAlive(utf16)
}
func paintHeader(hwnd uintptr, n *note) {
	var ps paintStruct
	dc := call(beginPaint, hwnd, uintptr(unsafe.Pointer(&ps)))
	if dc == 0 {
		return
	}
	defer call(endPaint, hwnd, uintptr(unsafe.Pointer(&ps)))
	var bounds rect
	call(getClientRect, hwnd, uintptr(unsafe.Pointer(&bounds)))
	p := palettes[n.theme]
	call(fillRect, dc, uintptr(unsafe.Pointer(&bounds)), p.surface)
	call(frameRect, dc, uintptr(unsafe.Pointer(&bounds)), p.border)
	call(setBkMode, dc, 1)
	oldFont := call(selectObject, dc, font)
	defer call(selectObject, dc, oldFont)
	title := rect{10, 1, bounds.right - 144, headerH - 1}
	if title.right < 10 {
		title.right = 10
	}
	text(dc, "Pinny", title, dtVCenter|dtSingleLine, p.mutedColor)
	labels := []string{"+", "◇", "⋯", "×"}
	if n.pinned {
		labels[1] = "◆"
	}
	widths := []int32{36, 36, 36, 36}
	left := bounds.right - 144
	for i, label := range labels {
		button := rect{left, 1, left + widths[i], headerH - 1}
		if n.hovered == i+1 {
			call(fillRect, dc, uintptr(unsafe.Pointer(&button)), p.hover)
		}
		color := p.inkColor
		if i == 3 {
			color = rgb(220, 55, 55)
		}
		text(dc, label, button, dtCenter|dtVCenter|dtSingleLine, color)
		left += widths[i]
	}
}
