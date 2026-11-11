package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var user = syscall.NewLazyDLL("user32.dll")
var kernel = syscall.NewLazyDLL("kernel32.dll")
var common = syscall.NewLazyDLL("comctl32.dll")
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
var roomBox, userBox, passwordBox, syncBox uintptr
var displayedTVs []TV
var settingsWindow uintptr

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
	r := callmsg(list, 0x100C, ^uintptr(0), 2)
	if int32(r) < 0 {
		return -1
	}
	return int(r)
}

// These definitions follow the Win32 common-control ABI (including pointer alignment).
type lvColumn struct {
	Mask                                                               uint32
	Fmt, Width                                                         int32
	Text                                                               *uint16
	TextMax, SubItem, Image, Order, MinWidth, DefaultWidth, IdealWidth int32
}
type lvItem struct {
	Mask             uint32
	Item, SubItem    int32
	State, StateMask uint32
	Text             *uint16
	TextMax, Image   int32
	Param            uintptr
	Indent, GroupID  int32
	Columns          uint32
	ColumnIndices    *uint32
	ColumnFormats    *int32
	Group            int32
}
type notifyHeader struct {
	Window, ID uintptr
	Code       uint32
}

func setCell(row, col int, value string) {
	item := lvItem{SubItem: int32(col), Text: w(value)}
	callmsg(list, 0x1074, uintptr(row), uintptr(unsafe.Pointer(&item)))
}
func initTable() {
	// Report view, one selected row, persistent selection, border and tab focus.
	list = control("SysListView32", "", 0x0081000D, 20, 166, 1020, 270, 20)
	callmsg(list, 0x1036, 0, 0x00010021) // full-row selection, grid lines, double buffering
	columns := []struct {
		name  string
		width int
	}{{"Status", 90}, {"Device name", 190}, {"Room", 140}, {"Type", 65}, {"IP address", 140}, {"MAC address", 190}, {"Reported power", 110}, {"Last result", 270}}
	for i, c := range columns {
		col := lvColumn{Mask: 7, Width: int32(c.width), Text: w(c.name)}
		callmsg(list, 0x1061, uintptr(i), uintptr(unsafe.Pointer(&col)))
	}
	images, _, _ := common.NewProc("ImageList_Create").Call(16, 16, 0x21, 3, 0)
	for _, color := range [][3]byte{{24, 140, 73}, {200, 48, 48}, {211, 153, 0}} {
		data := dotIcon(color)
		icon, _, _ := user.NewProc("CreateIconFromResourceEx").Call(uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), 1, 0x30000, 16, 16, 0)
		common.NewProc("ImageList_ReplaceIcon").Call(images, ^uintptr(0), icon)
		user.NewProc("DestroyIcon").Call(icon)
	}
	callmsg(list, 0x1003, 1, images) // LVSIL_SMALL; owned by the list view
}

// Native 32-bit DIB icon, with a matching transparency mask. No asset files at runtime.
func dotIcon(color [3]byte) []byte {
	const size = 16
	data := make([]byte, 40+size*size*4+size*4)
	binary.LittleEndian.PutUint32(data[0:], 40)
	binary.LittleEndian.PutUint32(data[4:], size)
	binary.LittleEndian.PutUint32(data[8:], size*2)
	binary.LittleEndian.PutUint16(data[12:], 1)
	binary.LittleEndian.PutUint16(data[14:], 32)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)-7.5, float64(y)-7.5
			p := 40 + (y*size+x)*4
			if dx*dx+dy*dy <= 36 {
				copy(data[p:p+4], []byte{color[2], color[1], color[0], 255})
			} else {
				data[40+size*size*4+y*4+x/8] |= 1 << uint(7-x%8)
			}
		}
	}
	return data
}
func update() {
	state.Lock()
	if !state.Dirty {
		state.Unlock()
		return
	}
	state.Dirty = false
	tvs := append([]TV(nil), state.TVs...)
	message, busy := state.Message, state.Busy
	if state.SyncStatus != "" {
		message += "\r\n" + state.SyncStatus
	}
	state.Unlock()
	focused, _, _ := user.NewProc("GetFocus").Call()
	editing := focused == namebox || focused == roomBox
	editName, editRoom := "", ""
	if editing {
		editName = textOf(namebox)
		editRoom = textOf(roomBox)
	}
	// List-view messages send synchronous notifications; never hold the state lock here.
	idx := selected()
	selectedID := ""
	if idx >= 0 && idx < len(displayedTVs) {
		selectedID = deviceID(displayedTVs[idx])
	}
	if selectedID != "" {
		idx = -1
		for i, t := range tvs {
			if deviceID(t) == selectedID {
				idx = i
				break
			}
		}
	}
	displayedTVs = tvs
	top := callmsg(list, 0x1027, 0, 0)
	callmsg(list, 0xB, 0, 0)
	callmsg(list, 0x1009, 0, 0)
	for row, t := range tvs {
		image, text := tvIndicator(t)
		item := lvItem{Mask: 3, Item: int32(row), Text: w(text), Image: int32(image)}
		callmsg(list, 0x104D, 0, uintptr(unsafe.Pointer(&item)))
		for col, value := range []string{t.Name, t.Room, deviceKind(t), t.IP, t.MAC, t.Power, t.Status} {
			setCell(row, col+1, value)
		}
	}
	if idx >= 0 && idx < len(tvs) {
		item := lvItem{State: 3, StateMask: 3}
		callmsg(list, 0x102B, uintptr(idx), uintptr(unsafe.Pointer(&item)))
	}
	if top < uintptr(len(tvs)) {
		callmsg(list, 0x1013, top, 0)
	}
	callmsg(list, 0xB, 1, 0)
	user.NewProc("InvalidateRect").Call(list, 0, 1)
	if editing {
		label(namebox, editName)
		label(roomBox, editRoom)
	}
	label(status, message)
	v := uintptr(1)
	if busy {
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
	tvs := append([]TV(nil), displayedTVs...)
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
			return fmt.Sprintf("Found %d Roku TVs. ", count) + discoverKasa()
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
			state.Lock()
			delete(state.Deleted, deviceID(t))
			state.Unlock()
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
		for i := range state.TVs {
			if deviceID(state.TVs[i]) == deviceID(tvs[idx]) {
				state.TVs[i].Name = name
				state.TVs[i].Room = strings.TrimSpace(textOf(roomBox))
				state.TVs[i].Updated = time.Now().UnixNano()
			}
		}
		state.Message = "Device name and room saved."
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
		runJob(func() string { return sendPowerAndRefresh(targets, cmd) })
	case 10:
		ip := strings.TrimSpace(textOf(ipbox))
		runJob(func() string {
			t, e := inspectKasa(ip)
			if e != nil {
				return e.Error()
			}
			state.Lock()
			delete(state.Deleted, deviceID(t))
			state.Unlock()
			upsert(t)
			return "Kasa plug added."
		})
	case 11, 12:
		room := strings.TrimSpace(textOf(roomBox))
		targets := roomTargets(room)
		if len(targets) == 0 {
			label(status, "Enter a room with assigned devices first.")
			return
		}
		cmd := "PowerOn"
		if id == 12 {
			cmd = "PowerOff"
		}
		r, _, _ := user.NewProc("MessageBoxW").Call(win, uintptr(unsafe.Pointer(w(fmt.Sprintf("Send %s to %d devices in %s?", cmd, len(targets), room)))), uintptr(unsafe.Pointer(w("Confirm room command"))), 0x24)
		if r == 6 {
			runJob(func() string { return sendPowerAndRefresh(targets, cmd) })
		}
	case 13:
		k := strings.TrimSpace(textOf(syncBox))
		if k != "" && len(k) < 24 {
			label(status, "Sync key must be at least 24 characters, or blank to disable.")
			return
		}
		if e := saveSecrets(textOf(userBox), textOf(passwordBox), k); e != nil {
			label(status, e.Error())
			return
		}
		if settingsWindow != 0 {
			user.NewProc("DestroyWindow").Call(settingsWindow)
		}
		runJob(func() string { return syncNow() })
	case 14:
		b := make([]byte, 16)
		if _, e := rand.Read(b); e == nil {
			label(syncBox, hex.EncodeToString(b))
			label(status, "New sync key generated. Save settings, then copy this key to your other PCs.")
		}
	case 15:
		runJob(periodicWork)
	case 16:
		openSettings()
	case 9:
		if idx < 0 || idx >= len(tvs) {
			return
		}
		r, _, _ := user.NewProc("MessageBoxW").Call(win, uintptr(unsafe.Pointer(w("Remove "+tvs[idx].Name+" from the saved list?"))), uintptr(unsafe.Pointer(w("Remove TV"))), 0x24)
		if r != 6 {
			return
		}
		state.Lock()
		if state.Deleted == nil {
			state.Deleted = map[string]int64{}
		}
		state.Deleted[deviceID(tvs[idx])] = time.Now().UnixNano()
		for i := range state.TVs {
			if deviceID(state.TVs[i]) == deviceID(tvs[idx]) {
				state.TVs = append(state.TVs[:i], state.TVs[i+1:]...)
				break
			}
		}
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
	case 0x4E: // WM_NOTIFY / LVN_ITEMCHANGED
		var header notifyHeader
		// lParam is a Win32-owned NMHDR pointer, valid for this notification only.
		kernel.NewProc("RtlMoveMemory").Call(uintptr(unsafe.Pointer(&header)), b, unsafe.Sizeof(header))
		if header.ID == 20 && int32(header.Code) == -101 {
			idx := selected()
			t := displayedTVs
			if idx >= 0 && idx < len(t) {
				label(namebox, t[idx].Name)
				label(roomBox, t[idx].Room)
			}
			return 0
		}
		return 0
	case 0x111:
		action(int(a & 0xffff))
		return 0
	case 0x113:
		if a == 2 {
			runJob(periodicWork)
		}
		update()
		return 0
	case 2:
		if h != win {
			settingsWindow = 0
			return 0
		}
		user.NewProc("PostQuitMessage").Call(0)
		return 0
	}
	r, _, _ := def.Call(h, uintptr(m), a, b)
	return r
}
func main() {
	runtime.LockOSThread()
	mutex, _, mutexErr := kernel.NewProc("CreateMutexW").Call(0, 0, uintptr(unsafe.Pointer(w("Local\\ChurchDeviceControl"))))
	if mutexErr == syscall.Errno(183) {
		return
	}
	if mutex != 0 {
		defer kernel.NewProc("CloseHandle").Call(mutex)
	}
	load()
	loadSecrets()
	user.NewProc("SetProcessDPIAware").Call()
	init := struct{ Size, Classes uint32 }{8, 1}
	common.NewProc("InitCommonControlsEx").Call(uintptr(unsafe.Pointer(&init)))
	instance, _, _ := kernel.NewProc("GetModuleHandleW").Call(0)
	cursor, _, _ := user.NewProc("LoadCursorW").Call(0, 32512)
	font, _, _ = gdi.NewProc("CreateFontW").Call(18, 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(w("Segoe UI"))))
	icon, _, _ := user.NewProc("LoadImageW").Call(instance, 1, 1, 32, 32, 0x8000)
	smallIcon, _, _ := user.NewProc("LoadImageW").Call(instance, 1, 1, 16, 16, 0x8000)
	class := w("ChurchRokuController")
	wc := wndclass{Proc: syscall.NewCallback(proc), Instance: instance, Cursor: cursor, Background: 16, Class: class, Icon: icon, SmallIcon: smallIcon}
	wc.Size = uint32(unsafe.Sizeof(wc))
	user.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc)))
	win, _, _ = create.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(w("Church Device Control"))), 0x00CA0000, 100, 100, 1080, 770, 0, 0, instance, 0)
	if win == 0 {
		return
	}
	control("STATIC", "Church Device Control", 0, 20, 16, 650, 30, 0)
	control("STATIC", "Roku TVs + Kasa plugs • Status and LAN settings refresh every minute", 0, 20, 51, 1020, 25, 0)
	button := func(title string, x, y, width, id int) {
		buttons = append(buttons, control("BUTTON", title, 0x10000, x, y, width, 34, id))
	}
	button("Discover devices", 20, 88, 160, 1)
	button("Refresh status", 192, 88, 160, 2)
	control("STATIC", "Status: green = on  •  red = off  •  yellow = unknown / pending / unreachable", 0, 20, 138, 1010, 24, 0)
	initTable()
	button("Selected ON", 20, 452, 160, 5)
	button("Selected OFF", 192, 452, 160, 6)
	button("All devices ON…", 680, 452, 170, 7)
	button("All devices OFF…", 862, 452, 178, 8)
	control("STATIC", "Device IP", 0, 20, 504, 120, 24, 0)
	ipbox = control("EDIT", "", 0x00810080, 145, 500, 190, 30, 21)
	button("Add Roku", 347, 498, 140, 3)
	control("STATIC", "Device name", 0, 20, 546, 120, 24, 0)
	namebox = control("EDIT", "", 0x00810080, 145, 542, 350, 30, 22)
	button("Save device", 507, 540, 140, 4)
	button("Remove selected", 862, 540, 178, 9)
	button("Add Kasa", 500, 498, 140, 10)
	control("STATIC", "Room", 0, 20, 588, 120, 24, 0)
	roomBox = control("EDIT", "", 0x00810080, 145, 584, 350, 30, 23)
	button("Room ON…", 507, 582, 140, 11)
	button("Room OFF…", 660, 582, 140, 12)
	control("STATIC", "Assign a room above, then Save device. Enter a room name above to control that room.", 0, 20, 626, 1020, 24, 0)
	button("Settings…", 365, 88, 150, 16)
	button("Sync + refresh", 530, 88, 170, 15)
	status = control("STATIC", "", 0, 20, 664, 1020, 65, 0)
	user.NewProc("SetTimer").Call(win, 1, 250, 0)
	update()
	user.NewProc("ShowWindow").Call(win, 1)
	user.NewProc("UpdateWindow").Call(win)
	startLAN()
	syncStatus(syncNow())
	user.NewProc("SetTimer").Call(win, 2, 60000, 0)
	refreshOnLaunch()
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

func deviceKind(t TV) string {
	if t.Kind == "kasa" {
		return "Kasa"
	}
	return "Roku"
}

func openSettings() {
	if settingsWindow != 0 {
		user.NewProc("SetForegroundWindow").Call(settingsWindow)
		return
	}
	instance, _, _ := kernel.NewProc("GetModuleHandleW").Call(0)
	settingsWindow, _, _ = create.Call(0, uintptr(unsafe.Pointer(w("ChurchRokuController"))), uintptr(unsafe.Pointer(w("Local Kasa credentials and LAN sync"))), 0x00CA0000, 150, 150, 820, 370, win, 0, instance, 0)
	add := func(class, title string, style uintptr, x, y, width, height, id int) uintptr {
		h, _, _ := create.Call(0, uintptr(unsafe.Pointer(w(class))), uintptr(unsafe.Pointer(w(title))), 0x50000000|style, uintptr(x), uintptr(y), uintptr(width), uintptr(height), settingsWindow, uintptr(id), instance, 0)
		callmsg(h, 0x30, font, 1)
		return h
	}
	add("STATIC", "Kasa email", 0, 20, 24, 130, 25, 0)
	userBox = add("EDIT", "", 0x00810080, 160, 20, 600, 30, 24)
	add("STATIC", "Kasa password", 0, 20, 68, 130, 25, 0)
	passwordBox = add("EDIT", "", 0x008100A0, 160, 64, 600, 30, 25)
	add("STATIC", "LAN sync key", 0, 20, 112, 130, 25, 0)
	syncBox = add("EDIT", "", 0x00810080, 160, 108, 600, 30, 26)
	add("STATIC", "Credentials are protected by Windows for this user and never synced.", 0, 20, 160, 750, 25, 0)
	add("STATIC", "Generate a sync key, save it, then copy the same key to the other PCs.", 0, 20, 192, 750, 25, 0)
	add("BUTTON", "Generate key", 0x10000, 20, 248, 180, 34, 14)
	add("BUTTON", "Save local settings", 0x10000, 215, 248, 200, 34, 13)
	secrets.Lock()
	label(userBox, secrets.User)
	label(passwordBox, secrets.Password)
	label(syncBox, secrets.SyncKey)
	secrets.Unlock()
	user.NewProc("ShowWindow").Call(settingsWindow, 1)
}
