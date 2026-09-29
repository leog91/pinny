//go:build windows

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

type guid struct {
	a    uint32
	b, c uint16
	d    [8]byte
}

var (
	ole32             = syscall.NewLazyDLL("ole32.dll")
	coInitialize      = ole32.NewProc("CoInitializeEx")
	coUninitialize    = ole32.NewProc("CoUninitialize")
	coCreateInstance  = ole32.NewProc("CoCreateInstance")
	coTaskMemFree     = ole32.NewProc("CoTaskMemFree")
	shellItemFromPath = syscall.NewLazyDLL("shell32.dll").NewProc("SHCreateItemFromParsingName")
	fileOpenClass     = guid{0xdc1c5a9c, 0xe88a, 0x4dde, [8]byte{0xa5, 0xa1, 0x60, 0xf8, 0x2a, 0x20, 0xae, 0xf7}}
	fileSaveClass     = guid{0xc0b4e2f3, 0xba21, 0x4773, [8]byte{0x8d, 0xba, 0x33, 0x5e, 0xc9, 0x46, 0xeb, 0x8b}}
	fileDialogIID     = guid{0x42f85136, 0xdb7e, 0x439c, [8]byte{0x85, 0xf1, 0xe4, 0x07, 0x5d, 0x13, 0x5f, 0xc8}}
	shellItemIID      = guid{0x43826d1e, 0xe718, 0x42ee, [8]byte{0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe}}
)

//go:uintptrescapes
func comCall(object uintptr, method int, args ...uintptr) uintptr {
	vtable := *(*uintptr)(unsafe.Pointer(object))
	function := *(*uintptr)(unsafe.Pointer(vtable + uintptr(method)*unsafe.Sizeof(uintptr(0))))
	parameters := append([]uintptr{object}, args...)
	result, _, _ := syscall.SyscallN(function, parameters...)
	return result
}

func comError(result uintptr) error {
	if int32(result) < 0 {
		return fmt.Errorf("Windows dialog error 0x%08X", uint32(result))
	}
	return nil
}

// Uses the native Common Item Dialog for JSON files and folders.
// An empty path with no error means the user cancelled.
func choosePath(owner uintptr, title, initialFolder string, save, folder bool) (string, error) {
	class := &fileOpenClass
	if save {
		class = &fileSaveClass
	}
	var dialog uintptr
	result := call(coCreateInstance, uintptr(unsafe.Pointer(class)), 0, 1,
		uintptr(unsafe.Pointer(&fileDialogIID)), uintptr(unsafe.Pointer(&dialog)))
	if err := comError(result); err != nil {
		return "", err
	}
	defer comCall(dialog, 2)               // Release.
	options := uintptr(0x40 | 0x8 | 0x800) // Filesystem, no directory change, existing path.
	if folder {
		options |= 0x20 // FOS_PICKFOLDERS.
	} else if save {
		options |= 0x2 // FOS_OVERWRITEPROMPT.
	} else {
		options |= 0x1000 // FOS_FILEMUSTEXIST.
	}
	if err := comError(comCall(dialog, 9, options)); err != nil {
		return "", err
	}
	if err := comError(comCall(dialog, 17, uintptr(unsafe.Pointer(wide(title))))); err != nil {
		return "", err
	}
	if !folder {
		type filterSpec struct{ name, pattern *uint16 }
		filters := []filterSpec{{wide("JSON files (*.json)"), wide("*.json")}, {wide("All files"), wide("*.*")}}
		result = comCall(dialog, 4, uintptr(len(filters)), uintptr(unsafe.Pointer(&filters[0])))
		runtime.KeepAlive(filters)
		if err := comError(result); err != nil {
			return "", err
		}
		if err := comError(comCall(dialog, 22, uintptr(unsafe.Pointer(wide("json"))))); err != nil {
			return "", err
		}
		if save {
			if err := comError(comCall(dialog, 15, uintptr(unsafe.Pointer(wide("Pinny-backup.json"))))); err != nil {
				return "", err
			}
		}
	}
	if initialFolder != "" {
		var item uintptr
		result = call(shellItemFromPath, uintptr(unsafe.Pointer(wide(initialFolder))), 0,
			uintptr(unsafe.Pointer(&shellItemIID)), uintptr(unsafe.Pointer(&item)))
		if comError(result) == nil {
			comCall(dialog, 12, item) // SetFolder.
			comCall(item, 2)
		}
	}
	result = comCall(dialog, 3, owner) // Show.
	if uint32(result) == 0x800704c7 {  // HRESULT_FROM_WIN32(ERROR_CANCELLED).
		return "", nil
	}
	if err := comError(result); err != nil {
		return "", err
	}
	var item uintptr
	if err := comError(comCall(dialog, 20, uintptr(unsafe.Pointer(&item)))); err != nil {
		return "", err
	}
	defer comCall(item, 2)
	var path *uint16
	if err := comError(comCall(item, 5, 0x80058000, uintptr(unsafe.Pointer(&path)))); err != nil {
		return "", err
	}
	defer call(coTaskMemFree, uintptr(unsafe.Pointer(path)))
	length := 0
	for *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(path)) + uintptr(length)*2)) != 0 {
		length++
	}
	return syscall.UTF16ToString(unsafe.Slice(path, length)), nil
}
