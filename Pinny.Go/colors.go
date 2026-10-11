//go:build windows

package main

var noteColors = []struct {
	id   uintptr
	name string
}{
	{lightID, "Light"},
	{darkID, "Dark"},
	{paperID, "Paper"},
	{yellowID, "Soft yellow"},
	{peachID, "Peach"},
	{roseID, "Rose"},
	{mintID, "Mint"},
	{lavenderID, "Lavender"},
}

func colorName(id uintptr) string {
	for _, option := range noteColors {
		if option.id == id {
			return option.name
		}
	}
	return ""
}

func updateColorMenu(n *note) {
	for _, option := range noteColors {
		var flags uintptr
		if option.name == n.theme {
			flags = mfChecked
		}
		call(checkMenuItem, n.colorMenu, option.id, flags)
	}
}
