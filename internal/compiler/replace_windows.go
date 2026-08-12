// SPDX-License-Identifier: AGPL-3.0-only
//go:build windows

package compiler

import (
	"os"
	"syscall"
	"unsafe"
)

var replaceFileW = syscall.NewLazyDLL("kernel32.dll").NewProc("ReplaceFileW")

// replaceFile uses ReplaceFileW when a destination exists because os.Rename is
// not an atomic replacement primitive on Windows. A new destination can use
// os.Rename: there is no last-good file whose visibility must be preserved.
func replaceFile(replacement, destination string) error {
	if _, err := os.Lstat(destination); err != nil {
		if os.IsNotExist(err) {
			return os.Rename(replacement, destination)
		}
		return err
	}
	destinationUTF16, err := syscall.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	replacementUTF16, err := syscall.UTF16PtrFromString(replacement)
	if err != nil {
		return err
	}
	result, _, callErr := replaceFileW.Call(
		uintptr(unsafe.Pointer(destinationUTF16)),
		uintptr(unsafe.Pointer(replacementUTF16)),
		0,
		1, // REPLACEFILE_WRITE_THROUGH
		0,
		0,
	)
	if result == 0 {
		return callErr
	}
	return nil
}
