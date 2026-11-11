package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptedSettingsMergeAndDeletion(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	state.TVs = []TV{{IP: "192.168.1.5", Name: "Lobby TV", Room: "Lobby", MAC: "AA:BB:CC:DD:EE:FF", Updated: 10, Power: "PowerOn", Status: "Reachable"}}
	state.Deleted = map[string]int64{}
	key := "0123456789abcdef0123456789abcdef"
	c := sharedConfig{Version: 1, Devices: []TV{{IP: "192.168.1.5", Name: "Front TV", Room: "Lobby", Updated: 20, Power: "PowerOff", Status: "Reachable"}}}
	b, e := sealConfig(c, key)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = openConfig(b, "wrong-key-0123456789abcdef"); e == nil {
		t.Fatal("accepted wrong key")
	}
	b[len(b)-1] ^= 1
	if _, e = openConfig(b, key); e == nil {
		t.Fatal("accepted tampered packet")
	}
	b[len(b)-1] ^= 1
	received, e := openConfig(b, key)
	if e != nil {
		t.Fatal(e)
	}
	if !mergeConfig(received) {
		t.Fatal("did not merge")
	}
	tv := snapshot()[0]
	if tv.Name != "Front TV" || tv.Power != "PowerOn" || tv.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Fatal(tv)
	}
	mergeConfig(sharedConfig{Version: 1, Devices: []TV{{IP: tv.IP, Name: "old", Updated: 5}}})
	if snapshot()[0].Name != "Front TV" {
		t.Fatal("stale overwrite")
	}
	mergeConfig(sharedConfig{Version: 1, Deleted: map[string]int64{deviceID(tv): 30}})
	mergeConfig(c)
	if len(snapshot()) != 0 {
		t.Fatal("deleted device resurrected")
	}
}
func TestWireSettingsExcludeCredentialsAndRuntime(t *testing.T) {
	state.TVs = []TV{{IP: "192.168.1.5", Name: "Device", Power: "PowerOn", Status: "Reachable"}}
	state.Deleted = nil
	state.Lock()
	c := configSnapshotLocked(true)
	state.Unlock()
	if c.Devices[0].Power != "" || c.Devices[0].Status != "" {
		t.Fatal("runtime data synced")
	}
	b, _ := json.Marshal(c)
	var m map[string]any
	json.Unmarshal(b, &m)
	if _, ok := m["Password"]; ok {
		t.Fatal("password in packet")
	}
}
func TestLegacyConfigMigrationAndRooms(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	os.MkdirAll(filepath.Dir(configPath()), 0700)
	if e := os.WriteFile(configPath(), []byte(`[{"IP":"192.168.1.5","Name":"Lobby (Cafe)"}]`), 0600); e != nil {
		t.Fatal(e)
	}
	state.TVs = nil
	load()
	tv := snapshot()[0]
	if tv.Name != "Lobby (Cafe)" || tv.Room != "" {
		t.Fatal(tv)
	}
	state.Lock()
	state.TVs[0].Room = "Lobby"
	state.TVs = append(state.TVs, TV{IP: "192.168.1.6", Name: "Lamp", Room: "Lobby", Kind: "kasa"})
	e := saveLocked()
	state.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	load()
	if len(roomTargets("lobby")) != 2 {
		t.Fatal(snapshot())
	}
}
