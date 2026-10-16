//go:build windows

package main

import (
	"fmt"
	"runtime"
	stdstrings "strings"
	"syscall"
	"testing"
	"unicode/utf16"
	"unsafe"
)

func TestWideCacheBounded(t *testing.T) {
	previous := strings
	strings = make(map[string]*uint16)
	t.Cleanup(func() { strings = previous })
	for i := 0; i < 1000; i++ {
		wide(fmt.Sprintf("Title %d", i))
		wide(stdstrings.Repeat("x", 4096))
	}
	if len(strings) > 128 {
		t.Fatalf("UTF-16 cache grew to %d entries", len(strings))
	}
	for key := range strings {
		if len(key) > 256 {
			t.Fatalf("cache retained %d-byte content", len(key))
		}
	}
	// Native tooltip text must remain valid even when the cache is full.
	hint := toolInfo{text: wide("uncached tooltip text 😀")}
	runtime.GC()
	if got := syscall.UTF16ToString(unsafe.Slice(hint.text, 25)); got != "uncached tooltip text 😀" {
		t.Fatalf("retained tooltip text: %q", got)
	}
}

func TestFindLinksOffsetsAcrossMatches(t *testing.T) {
	text := "😀 https://example.com/a_(b). www.example.org/😀 https:// https://example.net/end"
	links := findLinks(text)
	want := []string{"https://example.com/a_(b)", "www.example.org/😀", "https://example.net/end"}
	if len(links) != len(want) {
		t.Fatalf("got %d links, want %d", len(links), len(want))
	}
	for i, candidate := range want {
		at := stdstrings.Index(text, candidate)
		start := len(utf16.Encode([]rune(text[:at])))
		end := start + len(utf16.Encode([]rune(candidate)))
		if links[i].start != start || links[i].end != end {
			t.Errorf("link %d offsets: %+v, want %d..%d", i, links[i], start, end)
		}
	}
}

// Run with -run '^$' -bench 'Benchmark(WideUnique|FindLinksMany|NativeHoverLarge)' -benchmem.
func BenchmarkWideUnique(b *testing.B) {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := wide(fmt.Sprintf("%d:%s", i, stdstrings.Repeat("x", 4096)))
		runtime.KeepAlive(p)
	}
	b.StopTimer()
	runtime.GC()
	runtime.ReadMemStats(&after)
	b.ReportMetric(float64(int64(after.HeapAlloc)-int64(before.HeapAlloc)), "retained-B")
}

func BenchmarkFindLinksMany(b *testing.B) {
	text := stdstrings.Repeat("😀 padding https://example.com/path next\r\n", 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		findLinks(text)
	}
}

func BenchmarkNativeHoverLarge(b *testing.B) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	editor := call(createWindow, 0, uintptr(unsafe.Pointer(wide("EDIT"))), 0, wsPopup|esMultiline, 0, 0, 320, 320, 0, 0, 0, 0)
	if editor == 0 {
		b.Fatal("could not create editor")
	}
	defer call(destroyWindow, editor)
	text := stdstrings.Repeat("plain note text without links\r\n", 4000)
	call(sendMessage, editor, emLimitText, 0x7ffffffe, 0)
	call(setWindowText, editor, uintptr(unsafe.Pointer(wide(text))))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		editorLinkAt(editor, packedPoint(point{20, 20}))
	}
}
