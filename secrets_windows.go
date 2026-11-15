package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

type blob struct {
	Size uint32
	Data *byte
}

func crypt(b []byte, decrypt bool) ([]byte, error) {
	if len(b) == 0 || uint64(len(b)) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("invalid credential data size")
	}
	in := blob{Size: uint32(len(b)), Data: &b[0]}
	// Go zero-initializes this DATA_BLOB. DPAPI allocates out.Data; this
	// function copies the result before wiping and freeing that Windows buffer.
	var out blob
	proc := syscall.NewLazyDLL("crypt32.dll").NewProc("CryptProtectData")
	if decrypt {
		proc = syscall.NewLazyDLL("crypt32.dll").NewProc("CryptUnprotectData")
	}
	// The two DPAPI functions share this argument layout. Selecting proc above
	// selects encryption versus decryption; flag 1 prevents interactive prompts.
	r, _, e := proc.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(b)
	if out.Data != nil {
		defer func() {
			clear(unsafe.Slice(out.Data, out.Size))
			syscall.NewLazyDLL("kernel32.dll").NewProc("LocalFree").Call(uintptr(unsafe.Pointer(out.Data)))
		}()
	}
	if r == 0 {
		return nil, fmt.Errorf("Windows DPAPI failed: %w", e)
	}
	if out.Data == nil || out.Size == 0 {
		return nil, fmt.Errorf("Windows DPAPI returned no data")
	}
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
func protect(b []byte) ([]byte, error)   { return crypt(b, false) }
func unprotect(b []byte) ([]byte, error) { return crypt(b, true) }
