package main

import (
	"image"
	"image/png"
	"os"
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

// Run on a Windows runner: exercise the actual controls, notifications and resources.
func TestNativeTableAndIcon(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	init := struct{ Size, Classes uint32 }{8, 1}
	r, _, _ := common.NewProc("InitCommonControlsEx").Call(uintptr(unsafe.Pointer(&init)))
	if r == 0 {
		t.Fatal("InitCommonControlsEx failed")
	}
	instance, _, _ := kernel.NewProc("GetModuleHandleW").Call(0)
	icon, _, _ := user.NewProc("LoadImageW").Call(instance, 1, 1, 32, 32, 0)
	if icon == 0 {
		t.Fatal("embedded application icon not found")
	}
	defer user.NewProc("DestroyIcon").Call(icon)
	class := w("ChurchTVTestWindow")
	wc := wndclass{Proc: syscall.NewCallback(proc), Instance: instance, Class: class}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if r, _, _ := user.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		t.Fatal("register class failed")
	}
	win, _, _ = create.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(w("UI test"))), 0x00CA0000, 0, 0, 1080, 660, 0, 0, instance, 0)
	if win == 0 {
		t.Fatal("create window failed")
	}
	defer user.NewProc("DestroyWindow").Call(win)
	namebox = control("EDIT", "", 0, 0, 0, 200, 30, 22)
	status = control("STATIC", "", 0, 0, 40, 600, 30, 0)
	initTable()
	if list == 0 {
		t.Fatal("ListView creation failed")
	}
	state.TVs = []TV{{Name: "Lobby", IP: "192.168.1.5", Power: "PowerOn", Status: "Reachable"}, {Name: "Sanctuary", IP: "192.168.1.6", Power: "DisplayOff", Status: "Reachable"}, {Name: "Classroom", IP: "192.168.1.7", Status: "Unreachable"}}
	state.Dirty = true
	update()
	if count := callmsg(list, 0x1004, 0, 0); count != 3 {
		t.Fatalf("row count %d", count)
	}
	getCell := func(row, col int) string {
		buf := make([]uint16, 256)
		item := lvItem{SubItem: int32(col), Text: &buf[0], TextMax: int32(len(buf))}
		callmsg(list, 0x1073, uintptr(row), uintptr(unsafe.Pointer(&item)))
		return syscall.UTF16ToString(buf)
	}
	for _, c := range []struct {
		row, col int
		want     string
	}{{0, 0, "On"}, {0, 1, "Lobby"}, {0, 2, "192.168.1.5"}, {1, 0, "Off"}, {2, 0, "Unknown"}} {
		if got := getCell(c.row, c.col); got != c.want {
			t.Errorf("cell %d,%d = %q want %q", c.row, c.col, got, c.want)
		}
	}
	images := callmsg(list, 0x1002, 1, 0)
	count, _, _ := common.NewProc("ImageList_GetImageCount").Call(images)
	if count != 3 {
		t.Fatalf("indicator images: %d", count)
	}
	item := lvItem{State: 3, StateMask: 3}
	callmsg(list, 0x102B, 1, uintptr(unsafe.Pointer(&item)))
	if selected() != 1 || textOf(namebox) != "Sanctuary" {
		t.Fatalf("selection notification failed: %d %q", selected(), textOf(namebox))
	}
	state.Dirty = true
	update() // notifications must not reenter a held state mutex
	if selected() != 1 {
		t.Fatal("selection lost after refresh")
	}
	if header := callmsg(list, 0x101F, 0, 0); callmsg(header, 0x1200, 0, 0) != 5 {
		t.Fatal("table must contain five columns")
	}
	// Capture the real native table so the release can also receive visual inspection.
	user.NewProc("ShowWindow").Call(win, 1)
	user.NewProc("UpdateWindow").Call(win)
	dc, _, _ := user.NewProc("GetDC").Call(list)
	memory, _, _ := gdi.NewProc("CreateCompatibleDC").Call(dc)
	bitmap, _, _ := gdi.NewProc("CreateCompatibleBitmap").Call(dc, 1020, 228)
	old, _, _ := gdi.NewProc("SelectObject").Call(memory, bitmap)
	ok, _, _ := user.NewProc("PrintWindow").Call(list, memory, 2)
	gdi.NewProc("SelectObject").Call(memory, old)
	if ok != 0 {
		info := struct {
			Size                   uint32
			Width, Height          int32
			Planes, Bits           uint16
			Compression, ImageSize uint32
			XPels, YPels           int32
			Used, Important        uint32
		}{Size: 40, Width: 1020, Height: -228, Planes: 1, Bits: 32}
		pixels := make([]byte, 1020*228*4)
		lines, _, _ := gdi.NewProc("GetDIBits").Call(memory, bitmap, 0, 228, uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&info)), 0)
		if lines == 228 {
			img := image.NewNRGBA(image.Rect(0, 0, 1020, 228))
			for i := 0; i < len(pixels); i += 4 {
				img.Pix[i] = pixels[i+2]
				img.Pix[i+1] = pixels[i+1]
				img.Pix[i+2] = pixels[i]
				img.Pix[i+3] = 255
			}
			if e := os.MkdirAll("build", 0700); e != nil {
				t.Fatal(e)
			}
			f, e := os.Create("build/table-smoke.png")
			if e != nil {
				t.Fatal(e)
			}
			if e = png.Encode(f, img); e != nil {
				t.Fatal(e)
			}
			f.Close()
		}
	}
	gdi.NewProc("DeleteObject").Call(bitmap)
	gdi.NewProc("DeleteDC").Call(memory)
	user.NewProc("ReleaseDC").Call(list, dc)

}
