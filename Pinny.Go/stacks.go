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
	stackListID       = 102
	createStackID     = 40
	renameStackID     = 41
	popStackNoteID    = 42
	returnStackNoteID = 43
	standaloneNoteID  = 44
	firstStackMoveID  = 1000

	stackUpButton   = 5
	stackDownButton = 6
	stackArrowWidth = 24

	lbGetTopIndex   = 0x018e
	lbSetTopIndex   = 0x0197
	lbGetItemRect   = 0x0198
	lbItemFromPoint = 0x01a9
)

type noteStack struct {
	id, name string
	notes    []noteState
	active   int
	list     uintptr

	dragStart int32
	dragFrom  int
	dragTo    int
	dragging  bool
}

var (
	stackListCallback uintptr
	deleteMenuItem    = user32.NewProc("DeleteMenu")
	getMenuItemCount  = user32.NewProc("GetMenuItemCount")
)

// Keep a flat list of note records for compatibility with older note files
// and backups. File order determines the order within each stack.
func showSavedNotes(states []noteState) error {
	stacks := make(map[string]*noteStack)
	for _, state := range states {
		if state.Stack == nil || state.Stack.ID == "" {
			continue
		}
		stack := stacks[state.Stack.ID]
		if stack == nil {
			name := cleanTitle(state.Stack.Name)
			if name == "" {
				name = "Note stack"
			}
			stack = &noteStack{id: state.Stack.ID, name: name}
			stacks[stack.id] = stack
		}
		if state.ID == "" {
			id, err := newNoteID()
			if err != nil {
				return err
			}
			state.ID = id
		}
		if state.Stack.Active {
			stack.active = len(stack.notes)
		}
		stack.notes = append(stack.notes, state)
	}
	for _, state := range states {
		if state.Stack == nil || state.Stack.ID == "" {
			if err := showSavedNote(state); err != nil {
				return err
			}
			continue
		}
		stack := stacks[state.Stack.ID]
		if stack == nil {
			continue
		}
		delete(stacks, stack.id)
		input := savedNoteInput(stack.notes[stack.active])
		input.stack = stack
		if err := showNote(input); err != nil {
			return err
		}
	}
	return nil
}

func activeNoteState(n *note) noteState {
	return noteState{ID: n.id, Title: currentTitle(n), Text: editorText(n.edit), Theme: n.theme, ReturnTo: n.returnTo, noteDates: n.dates}
}

func snapshotWindow(hwnd uintptr, n *note) ([]noteState, error) {
	var bounds rect
	if call(getWindowRect, hwnd, uintptr(unsafe.Pointer(&bounds))) == 0 {
		return nil, fmt.Errorf("could not read note position")
	}
	current := activeNoteState(n)
	states := []noteState{current}
	if stack := n.stack; stack != nil {
		states = append([]noteState(nil), stack.notes...)
		states[stack.active] = current
		for i := range states {
			states[i].Stack = &stackMembership{ID: stack.id, Name: stack.name, Active: i == stack.active}
		}
	}
	for i := range states {
		state := &states[i]
		state.Left, state.Top = float64(bounds.left), float64(bounds.top)
		state.Width, state.Height = float64(bounds.right-bounds.left), float64(bounds.bottom-bounds.top)
		state.IsPinned = n.pinned
	}
	return states, nil
}

func createStackList(hwnd uintptr, n *note) error {
	stack := n.stack
	if stackListCallback == 0 {
		stackListCallback = syscall.NewCallback(stackListProc)
	}
	stack.list = viewerControl("LISTBOX", "", wsBorder|wsVScroll|wsTabStop|lbsNotify|0x0100, 0,
		hwnd, stackListID, call(getModuleHandle, 0)) // LBS_NOINTEGRALHEIGHT
	if stack.list == 0 {
		return fmt.Errorf("could not create the stack's note list")
	}
	call(sendMessage, stack.list, wmSetFont, font, 1)
	if call(setWindowSubclass, stack.list, stackListCallback, 1, hwnd) == 0 {
		call(destroyWindow, stack.list)
		stack.list = 0
		return fmt.Errorf("could not configure the stack's note list")
	}
	refreshStackList(n)
	return nil
}

func stackNoteLabel(state noteState) string {
	if title := cleanTitle(state.Title); title != "" {
		return title
	}
	for remaining := state.Text; remaining != ""; {
		line, rest, _ := stdstrings.Cut(remaining, "\n")
		if title := cleanTitle(line); title != "" {
			return title
		}
		remaining = rest
	}
	return "Untitled note"
}

func refreshStackList(n *note) {
	stack := n.stack
	top := int(call(sendMessage, stack.list, lbGetTopIndex, 0, 0))
	call(sendMessage, stack.list, 0x000b, 0, 0) // WM_SETREDRAW
	call(sendMessage, stack.list, lbResetContent, 0, 0)
	for _, state := range stack.notes {
		call(sendMessage, stack.list, lbAddString, 0, uintptr(unsafe.Pointer(wide(stackNoteLabel(state)))))
	}
	call(sendMessage, stack.list, lbSetTopIndex, uintptr(max(0, min(top, len(stack.notes)-1))), 0)
	call(sendMessage, stack.list, lbSetCurSel, uintptr(stack.active), 0)
	call(sendMessage, stack.list, 0x000b, 1, 0)
	call(invalidateRect, stack.list, 0, 1)
}

func stashStackNote(hwnd uintptr, n *note) {
	finishRename(hwnd, n, true)
	n.stack.notes[n.stack.active] = activeNoteState(n)
}

func refreshStackLabel(n *note) {
	stack := n.stack
	old := stack.notes[stack.active]
	if old.Title != "" {
		return
	}
	current := activeNoteState(n)
	stack.notes[stack.active] = current
	if stackNoteLabel(old) != stackNoteLabel(current) {
		refreshStackList(n)
	}
}

func loadStackNote(hwnd uintptr, n *note, index int) {
	stack := n.stack
	stack.active = index
	state := stack.notes[index]
	n.id, n.title, n.theme, n.returnTo = state.ID, cleanTitle(state.Title), savedNoteInput(state).theme, state.ReturnTo
	n.dates = state.noteDates
	n.loading = true
	call(setWindowText, n.edit, uintptr(unsafe.Pointer(wide(state.Text))))
	call(sendMessage, n.edit, emSetSel, 0, 0)
	n.loading = false
	updateColorMenu(n)
	refreshStackList(n)
	call(invalidateRect, n.edit, 0, 1)
	call(invalidateRect, hwnd, 0, 0)
	queueSave(hwnd)
}

func selectStackNote(hwnd uintptr, n *note, index int) {
	if index < 0 || index >= len(n.stack.notes) || index == n.stack.active {
		return
	}
	stashStackNote(hwnd, n)
	loadStackNote(hwnd, n, index)
}

func newStackNote(hwnd uintptr, n *note) {
	id, err := newNoteID()
	if err != nil {
		showOperationError(hwnd, "New note", err)
		return
	}
	stashStackNote(hwnd, n)
	n.stack.notes = append(n.stack.notes, noteState{ID: id, Theme: n.theme, noteDates: initialNoteDates(time.Now())})
	loadStackNote(hwnd, n, len(n.stack.notes)-1)
	call(setFocus, n.edit)
}

func moveStackNote(hwnd uintptr, n *note, to int) {
	stack := n.stack
	from := stack.active
	if to < 0 || to >= len(stack.notes) || from == to {
		return
	}
	stashStackNote(hwnd, n)
	state := stack.notes[from]
	if from < to {
		copy(stack.notes[from:to], stack.notes[from+1:to+1])
	} else {
		copy(stack.notes[to+1:from+1], stack.notes[to:from])
	}
	stack.notes[to] = state
	stack.active = to
	refreshStackList(n)
	call(invalidateRect, hwnd, 0, 0)
	queueSave(hwnd)
}

func stackRowAt(list, lp uintptr) int {
	row := call(sendMessage, list, lbItemFromPoint, 0, lp)
	if row>>16 != 0 {
		return -1
	}
	index := int(row & 0xffff)
	var item rect
	if call(sendMessage, list, lbGetItemRect, uintptr(index), uintptr(unsafe.Pointer(&item))) == ^uintptr(0) ||
		high(lp) < item.top || high(lp) >= item.bottom {
		return -1
	}
	return index
}

func stackListProc(list, message, wp, lp, id, owner uintptr) uintptr {
	if message == 0x0082 { // WM_NCDESTROY
		call(removeWindowSubclass, list, stackListCallback, id)
	}
	n := windows[owner]
	if n == nil || n.stack == nil {
		return call(defSubclassProc, list, message, wp, lp)
	}
	stack := n.stack
	switch message {
	case wmLButtonDown:
		index := stackRowAt(list, lp)
		if index >= 0 && index < len(stack.notes) {
			call(setFocus, list)
			selectStackNote(owner, n, index)
			stack.dragStart = high(lp)
			stack.dragFrom, stack.dragTo, stack.dragging = index, index, false
			call(setCapture, list)
			return 0
		}
	case wmMouseMove:
		if wp&1 != 0 && call(stackGetCapture) == list {
			delta := high(lp) - stack.dragStart
			stack.dragging = stack.dragging || delta >= 4 || delta <= -4
			if stack.dragging {
				target := stackRowAt(list, lp)
				if target != stack.dragTo {
					stack.dragTo = target
					call(invalidateRect, list, 0, 1)
				}
			}
			return 0
		}
	case wmLButtonUp:
		if call(stackGetCapture) == list {
			target, dragging := stackRowAt(list, lp), stack.dragging
			call(releaseCapture)
			if dragging {
				moveStackNote(owner, n, target)
			}
			return 0
		}
	case 0x0215: // WM_CAPTURECHANGED
		stack.dragging = false
		call(invalidateRect, list, 0, 1)
	case 0x0100: // WM_KEYDOWN
		if call(getKeyState, 0x11)&0x8000 != 0 {
			switch wp {
			case 0x26: // Up
				moveStackNote(owner, n, stack.active-1)
				return 0
			case 0x28: // Down
				moveStackNote(owner, n, stack.active+1)
				return 0
			}
		}
	case wmPaint:
		result := call(defSubclassProc, list, message, wp, lp)
		if stack.dragging && stack.dragTo >= 0 && stack.dragTo != stack.dragFrom {
			paintStackDropLine(list, stack, palettes[n.theme])
		}
		return result
	}
	return call(defSubclassProc, list, message, wp, lp)
}

var stackGetCapture = user32.NewProc("GetCapture")

func paintStackDropLine(list uintptr, stack *noteStack, p palette) {
	var item rect
	if call(sendMessage, list, lbGetItemRect, uintptr(stack.dragTo), uintptr(unsafe.Pointer(&item))) == ^uintptr(0) {
		return
	}
	if stack.dragTo > stack.dragFrom {
		item.top = item.bottom - 2
	}
	item.bottom = item.top + 2
	dc := call(getDC, list)
	if dc != 0 {
		call(fillRect, dc, uintptr(unsafe.Pointer(&item)), p.border)
		call(releaseDC, list, dc)
	}
}

func headerButtonsWidth(n *note) int32 {
	if n.stack != nil {
		return 144 + 2*stackArrowWidth
	}
	return 144
}

func paintStackArrows(dc uintptr, right int32, n *note, p palette) {
	left := right - headerButtonsWidth(n)
	for i, label := range []string{"↑", "↓"} {
		button := rect{left, 1, left + stackArrowWidth, headerH - 1}
		color := p.inkColor
		target := n.stack.active + 2*i - 1
		if target < 0 || target >= len(n.stack.notes) {
			color = p.mutedColor
		} else if n.hovered == stackUpButton+i {
			call(fillRect, dc, uintptr(unsafe.Pointer(&button)), p.hover)
		}
		text(dc, label, button, dtCenter|dtVCenter|dtSingleLine, color)
		left += stackArrowWidth
	}
}

func appendStackMenuItem(menu, id uintptr, label string) {
	call(appendMenu, menu, 0, id, uintptr(unsafe.Pointer(wide(stdstrings.ReplaceAll(label, "&", "&&")))))
}

func refreshStackMenu(hwnd uintptr, n *note) {
	menu := n.stackMenu
	for count := int32(call(getMenuItemCount, menu)); count > 0; count-- {
		call(deleteMenuItem, menu, 0, 0x0400) // MF_BYPOSITION
	}
	n.stackTargets = n.stackTargets[:0]
	if n.stack != nil {
		appendStackMenuItem(menu, renameStackID, "Rename stack...")
		appendStackMenuItem(menu, popStackNoteID, "Pop out this note")
		appendStackMenuItem(menu, standaloneNoteID, "New standalone note")
		return
	}
	if n.returnTo != nil {
		name := n.returnTo.Name
		if target := findStackWindow(n.returnTo.ID); target != 0 {
			name = windows[target].stack.name
		}
		appendStackMenuItem(menu, returnStackNoteID, "Move back to "+name)
	}
	appendStackMenuItem(menu, createStackID, "Create stack from this note")
	for _, target := range windowOrder {
		if other := windows[target]; target != hwnd && other.stack != nil {
			appendStackMenuItem(menu, firstStackMoveID+uintptr(len(n.stackTargets)), "Move to "+other.stack.name)
			n.stackTargets = append(n.stackTargets, target)
		}
	}
}

func findStackWindow(id string) uintptr {
	for _, hwnd := range windowOrder {
		if stack := windows[hwnd].stack; stack != nil && stack.id == id {
			return hwnd
		}
	}
	return 0
}

func handleStackCommand(hwnd uintptr, n *note, id uintptr) {
	switch {
	case id == createStackID && n.stack == nil:
		name := "Note stack"
		if n.title != "" {
			name = cleanTitle(n.title + " stack")
		}
		createStack(hwnd, n, &stackPosition{Name: name})
	case id == returnStackNoteID && n.stack == nil && n.returnTo != nil:
		if target := findStackWindow(n.returnTo.ID); target != 0 {
			moveToStack(hwnd, n, target, n.returnTo.Index)
		} else {
			createStack(hwnd, n, n.returnTo)
		}
	case id == renameStackID && n.stack != nil:
		beginStackRename(hwnd, n)
	case id == popStackNoteID && n.stack != nil:
		popStackNote(hwnd, n)
	case id == standaloneNoteID:
		newStandaloneNote(hwnd, n)
	case id >= firstStackMoveID && id-firstStackMoveID < uintptr(len(n.stackTargets)) && n.stack == nil:
		target := n.stackTargets[id-firstStackMoveID]
		moveToStack(hwnd, n, target, -1)
	}
}

func createStack(hwnd uintptr, n *note, position *stackPosition) {
	finishRename(hwnd, n, true)
	id := position.ID
	if id == "" {
		var err error
		id, err = newNoteID()
		if err != nil {
			showOperationError(hwnd, "Create stack", err)
			return
		}
	}
	state := activeNoteState(n)
	state.ReturnTo = nil
	name := cleanTitle(position.Name)
	if name == "" {
		name = "Note stack"
	}
	n.stack = &noteStack{id: id, name: name, notes: []noteState{state}}
	if err := createStackList(hwnd, n); err != nil {
		n.stack = nil
		showOperationError(hwnd, "Create stack", err)
		return
	}
	n.returnTo = nil
	updateWindowTitle(hwnd, n)
	resizeEdit(hwnd, n)
	call(invalidateRect, hwnd, 0, 1)
	queueSave(hwnd)
}

func moveToStack(hwnd uintptr, n *note, target uintptr, index int) {
	other := windows[target]
	if other == nil || other.stack == nil {
		return
	}
	finishRename(hwnd, n, true)
	state := activeNoteState(n)
	state.ReturnTo = nil
	stashStackNote(target, other)
	stack := other.stack
	if index < 0 || index > len(stack.notes) {
		index = len(stack.notes)
	}
	stack.notes = slices.Insert(stack.notes, index, state)
	loadStackNote(target, other, index)
	call(destroyWindow, hwnd)
	call(showWindow, target, 9) // SW_RESTORE
	call(setForeground, target)
	call(setFocus, other.edit)
}

func popStackNote(hwnd uintptr, n *note) {
	finishRename(hwnd, n, true)
	states, err := snapshotWindow(hwnd, n)
	if err != nil {
		showOperationError(hwnd, "Pop out note", err)
		return
	}
	stack := n.stack
	state := states[stack.active]
	state.ReturnTo = &stackPosition{ID: stack.id, Name: stack.name, Index: stack.active}
	if len(stack.notes) == 1 {
		call(destroyWindow, stack.list)
		n.stack, n.returnTo = nil, state.ReturnTo
		updateWindowTitle(hwnd, n)
		resizeEdit(hwnd, n)
		call(invalidateRect, hwnd, 0, 1)
		call(setFocus, n.edit)
		queueSave(hwnd)
		return
	}
	input := savedNoteInput(state)
	input.x += 32
	input.y += 32
	if err := showNote(input); err != nil {
		showOperationError(hwnd, "Pop out note", err)
		return
	}
	removeStackNote(hwnd, n)
}

func removeStackNote(hwnd uintptr, n *note) {
	stack := n.stack
	stack.notes = slices.Delete(stack.notes, stack.active, stack.active+1)
	loadStackNote(hwnd, n, min(stack.active, len(stack.notes)-1))
}
