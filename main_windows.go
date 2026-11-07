package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var user = syscall.NewLazyDLL("user32.dll")
var kernel = syscall.NewLazyDLL("kernel32.dll")
var gdi = syscall.NewLazyDLL("gdi32.dll")
var create = user.NewProc("CreateWindowExW")
var def = user.NewProc("DefWindowProcW")
var send = user.NewProc("SendMessageW")
var setText = user.NewProc("SetWindowTextW")
var getText = user.NewProc("GetWindowTextW")
var enable = user.NewProc("EnableWindow")
var win, list, ipbox, namebox, status uintptr
var buttons []uintptr
var font uintptr

func w(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func callmsg(h uintptr, m uintptr, a uintptr, b uintptr) uintptr {
	r, _, _ := send.Call(h, m, a, b)
	return r
}
func label(h uintptr, s string) { setText.Call(h, uintptr(unsafe.Pointer(w(s)))) }
func textOf(h uintptr) string {
	b := make([]uint16, 512)
	getText.Call(h, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	return syscall.UTF16ToString(b)
}
func control(class, title string, style uintptr, x, y, width, height, id int) uintptr {
	h, _, _ := create.Call(0, uintptr(unsafe.Pointer(w(class))), uintptr(unsafe.Pointer(w(title))), 0x50000000|style, uintptr(x), uintptr(y), uintptr(width), uintptr(height), win, uintptr(id), 0, 0)
	callmsg(h, 0x30, font, 1)
	return h
}
func selected() int {
	r := callmsg(list, 0x188, 0, 0)
	if int32(r) < 0 {
		return -1
	}
	return int(r)
}
func update() {
	state.Lock()
	defer state.Unlock()
	if !state.Dirty {
		return
	}
	state.Dirty = false
	idx := selected()
	callmsg(list, 0xB, 0, 0)
	callmsg(list, 0x184, 0, 0)
	for _, t := range state.TVs {
		line := fmt.Sprintf("%s   |   %s   |   %s   |   %s", t.Name, t.IP, t.Power, t.Status)
		callmsg(list, 0x180, 0, uintptr(unsafe.Pointer(w(line))))
	}
	if idx >= 0 && idx < len(state.TVs) {
		callmsg(list, 0x186, uintptr(idx), 0)
	}
	callmsg(list, 0xB, 1, 0)
	user.NewProc("InvalidateRect").Call(list, 0, 1)
	label(status, state.Message)
	v := uintptr(1)
	if state.Busy {
		v = 0
	}
	for _, b := range buttons {
		enable.Call(b, v)
	}
}
func action(id int) {
	state.Lock()
	busy := state.Busy
	state.Unlock()
	if busy {
		return
	}
	tvs := snapshot()
	idx := selected()
	switch id {
	case 1:
		runJob(func() string {
			ips, e := discover()
			if e != nil {
				return e.Error()
			}
			count := 0
			for _, ip := range ips {
				t, e := inspect(ip)
				if e == nil {
					upsert(t)
					count++
				}
			}
			return fmt.Sprintf("Found %d Roku TVs. If a TV is missing, add its IP address below.", count)
		})
	case 2:
		runJob(refresh)
	case 3:
		ip := textOf(ipbox)
		runJob(func() string {
			t, e := inspect(ip)
			if e != nil {
				return "Could not add TV: " + e.Error()
			}
			upsert(t)
			return "TV added. Select it and save a room name below."
		})
	case 4:
		if idx < 0 || idx >= len(tvs) {
			return
		}
		name := textOf(namebox)
		if name == "" {
			return
		}
		state.Lock()
		state.TVs[idx].Name = name
		state.Message = "Room name saved."
		if e := saveLocked(); e != nil {
			state.Message = e.Error()
		}
		state.Dirty = true
		state.Unlock()
	case 5, 6, 7, 8:
		targets := tvs
		if id == 5 || id == 6 {
			if idx < 0 || idx >= len(tvs) {
				label(status, "Select a TV first.")
				return
			}
			targets = []TV{tvs[idx]}
		}
		if len(targets) == 0 {
			return
		}
		cmd := "PowerOn"
		if id == 6 || id == 8 {
			cmd = "PowerOff"
		}
		if id == 7 || id == 8 {
			r, _, _ := user.NewProc("MessageBoxW").Call(win, uintptr(unsafe.Pointer(w(fmt.Sprintf("Send %s to all %d saved TVs?", cmd, len(targets))))), uintptr(unsafe.Pointer(w("Confirm group command"))), 0x24)
			if r != 6 {
				return
			}
		}
		runJob(func() string { return sendPower(targets, cmd) })
	case 9:
		if idx < 0 || idx >= len(tvs) {
			return
		}
		r, _, _ := user.NewProc("MessageBoxW").Call(win, uintptr(unsafe.Pointer(w("Remove "+tvs[idx].Name+" from the saved list?"))), uintptr(unsafe.Pointer(w("Remove TV"))), 0x24)
		if r != 6 {
			return
		}
		state.Lock()
		state.TVs = append(state.TVs[:idx], state.TVs[idx+1:]...)
		state.Dirty = true
		state.Message = "TV removed."
		if e := saveLocked(); e != nil {
			state.Message = e.Error()
		}
		state.Unlock()
	}
	update()
}

type wndclass struct {
	Size       uint32
	Style      uint32
	Proc       uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	Menu       *uint16
	Class      *uint16
	SmallIcon  uintptr
}
type point struct{ X, Y int32 }
type message struct {
	Hwnd           uintptr
	Message        uint32
	Wparam, Lparam uintptr
	Time           uint32
	Pt             point
	Private        uint32
}

func proc(h uintptr, m uint32, a, b uintptr) uintptr {
	switch m {
	case 0x111:
		if a&0xffff == 20 && a>>16 == 1 {
			idx := selected()
			t := snapshot()
			if idx >= 0 && idx < len(t) {
				label(namebox, t[idx].Name)
			}
			return 0
		}
		action(int(a & 0xffff))
		return 0
	case 0x113:
		update()
		return 0
	case 2:
		user.NewProc("PostQuitMessage").Call(0)
		return 0
	}
	r, _, _ := def.Call(h, uintptr(m), a, b)
	return r
}
func main() {
	runtime.LockOSThread()
	load()
	user.NewProc("SetProcessDPIAware").Call()
	instance, _, _ := kernel.NewProc("GetModuleHandleW").Call(0)
	cursor, _, _ := user.NewProc("LoadCursorW").Call(0, 32512)
	font, _, _ = gdi.NewProc("CreateFontW").Call(18, 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(w("Segoe UI"))))
	class := w("ChurchRokuController")
	wc := wndclass{Proc: syscall.NewCallback(proc), Instance: instance, Cursor: cursor, Background: 16, Class: class}
	wc.Size = uint32(unsafe.Sizeof(wc))
	user.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc)))
	win, _, _ = create.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(w("Church TV Control — Roku"))), 0x00CA0000, 100, 100, 1080, 660, 0, 0, instance, 0)
	if win == 0 {
		return
	}
	control("STATIC", "Church TV Control", 0, 20, 16, 650, 30, 0)
	control("STATIC", "Same network as the TVs • Fast TV Start is required for reliable network power-on", 0, 20, 51, 1020, 25, 0)
	button := func(title string, x, y, width, id int) {
		buttons = append(buttons, control("BUTTON", title, 0x10000, x, y, width, 34, id))
	}
	button("Discover TVs", 20, 88, 160, 1)
	button("Refresh status", 192, 88, 160, 2)
	control("STATIC", "Room name  |  IP address  |  Reported power  |  Last result", 0, 20, 138, 1010, 24, 0)
	list = control("LISTBOX", "", 0x00A10001, 20, 166, 1020, 228, 20)
	button("Selected ON", 20, 410, 160, 5)
	button("Selected OFF", 192, 410, 160, 6)
	button("All TVs ON…", 680, 410, 170, 7)
	button("All TVs OFF…", 862, 410, 178, 8)
	control("STATIC", "TV IP address", 0, 20, 464, 120, 24, 0)
	ipbox = control("EDIT", "", 0x00810080, 145, 460, 190, 30, 21)
	button("Add by IP", 347, 458, 140, 3)
	control("STATIC", "Room name", 0, 20, 506, 120, 24, 0)
	namebox = control("EDIT", "", 0x00810080, 145, 502, 350, 30, 22)
	button("Save name", 507, 500, 140, 4)
	button("Remove selected", 862, 500, 178, 9)
	status = control("STATIC", "", 0, 20, 550, 1020, 65, 0)
	user.NewProc("SetTimer").Call(win, 1, 250, 0)
	update()
	user.NewProc("ShowWindow").Call(win, 1)
	user.NewProc("UpdateWindow").Call(win)
	var msg message
	for {
		r, _, _ := user.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if r == 0 || int32(r) == -1 {
			break
		}
		user.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&msg)))
		user.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
	}
}
