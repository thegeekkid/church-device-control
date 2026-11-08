package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

type blob struct {
	Size uint32
	Data *byte
}

func crypt(b []byte, decrypt bool) ([]byte, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("empty credentials")
	}
	in := blob{Size: uint32(len(b)), Data: &b[0]}
	var out blob
	proc := syscall.NewLazyDLL("crypt32.dll").NewProc("CryptProtectData")
	var r uintptr
	var e error
	if decrypt {
		proc = syscall.NewLazyDLL("crypt32.dll").NewProc("CryptUnprotectData")
		r, _, e = proc.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	} else {
		r, _, e = proc.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	}
	if r == 0 {
		return nil, e
	}
	defer syscall.NewLazyDLL("kernel32.dll").NewProc("LocalFree").Call(uintptr(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
func protect(b []byte) ([]byte, error)   { return crypt(b, false) }
func unprotect(b []byte) ([]byte, error) { return crypt(b, true) }
