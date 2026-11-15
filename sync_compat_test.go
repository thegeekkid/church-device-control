package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/json"
	"os"
	"testing"
)

// Frozen pre-MAC/pre-Disabled receiver shape, matching the v0.3.0 reader.
type legacyDevice03 struct {
	IP, Name, Model, Serial, Power, Status, Kind, Room string
	Updated                                            int64
}
type legacySettings03 struct {
	Version int
	Devices []legacyDevice03
	Deleted map[string]int64
	Request bool
}

func legacyRead03(packet []byte, key string) (legacySettings03, error) {
	var old legacySettings03
	h := sha256.Sum256([]byte(key))
	block, _ := aes.NewCipher(h[:])
	g, _ := cipher.NewGCM(block)
	b, e := g.Open(nil, packet[:g.NonceSize()], packet[g.NonceSize():], []byte("church-device-sync-v1"))
	if e != nil {
		return old, e
	}
	e = json.Unmarshal(b, &old)
	if old.Version != 1 {
		t := &json.UnmarshalTypeError{Value: "unsupported wire version"}
		return old, t
	}
	return old, e
}
func TestMixedVersionRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	const key = "0123456789abcdef0123456789abcdef"
	secrets.Lock()
	previous := secrets.SyncKey
	secrets.SyncKey = key
	secrets.Unlock()
	defer func() { secrets.Lock(); secrets.SyncKey = previous; secrets.Unlock() }()
	state.TVs = []TV{{IP: "192.0.2.20", Name: "New name", Room: "Lobby", Kind: "kasa", MAC: "AA:BB:CC:DD:EE:FF", Disabled: true, DisabledUpdated: 90, Updated: 100, Extras: map[string]json.RawMessage{"FutureOption": json.RawMessage(`{"enabled":true,"revision":50}`)}}}
	state.Deleted = map[string]int64{"roku:removed": 80}
	packet, e := syncPacket(true)
	if e != nil {
		t.Fatal(e)
	}
	old, e := legacyRead03(packet, key)
	if e != nil {
		t.Fatal(e)
	}
	if old.Devices[0].Name != "New name" || old.Devices[0].Room != "Lobby" || old.Deleted["roku:removed"] != 80 {
		t.Fatal(old)
	}
	// The old application changes a common field, dropping unknown fields exactly
	// as its original Go decoder/encoder does.
	old.Devices[0].Room = "Sanctuary"
	old.Devices[0].Updated = 200
	b, _ := json.Marshal(old)
	var decoded sharedConfig
	if e = json.Unmarshal(b, &decoded); e != nil {
		t.Fatal(e)
	}
	mergeConfig(decoded)
	got := snapshot()[0]
	if got.Room != "Sanctuary" || !got.Disabled || got.DisabledUpdated != 90 || got.MAC != "AA:BB:CC:DD:EE:FF" || len(got.Extras) == 0 {
		t.Fatalf("legacy edit erased new settings: %+v", got)
	}
	// v0.4.0 still gets its version-2 packet for the disabled flag.
	bridge, e := syncPacketVersion(false, 2)
	if e != nil {
		t.Fatal(e)
	}
	c, e := openConfig(bridge, key)
	if e != nil || c.Version != 2 || !c.Devices[0].Disabled {
		t.Fatal(c, e)
	}
}
func TestIndependentFeatureRevisionsAndFutureFields(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	state.Deleted = nil
	state.TVs = []TV{{IP: "192.0.2.21", Name: "Newer room edit", Room: "Lobby", Updated: 300, DisabledUpdated: 10}}
	c := sharedConfig{Version: 1, Capabilities: []string{"disabled-v1"}, Devices: []TV{{IP: "192.0.2.21", Name: "Old name", Updated: 100, Disabled: true, DisabledUpdated: 200}}}
	mergeConfig(c)
	got := snapshot()[0]
	if got.Name != "Newer room edit" || !got.Disabled || got.DisabledUpdated != 200 {
		t.Fatal(got)
	}
	c.Devices[0].Disabled = false
	c.Devices[0].DisabledUpdated = 150
	mergeConfig(c)
	if !snapshot()[0].Disabled {
		t.Fatal("stale feature value won")
	}
	c.Devices[0].DisabledUpdated = 250
	mergeConfig(c)
	if snapshot()[0].Disabled {
		t.Fatal("new feature value ignored")
	}
	var future TV
	if e := json.Unmarshal([]byte(`{"IP":"192.0.2.22","Name":"Future device","Updated":1,"FutureSchedule":{"mode":"weekly"}}`), &future); e != nil {
		t.Fatal(e)
	}
	state.TVs = []TV{future}
	state.Lock()
	e := saveLocked()
	state.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	state.TVs = nil
	load()
	upsert(TV{IP: future.IP, Name: "discovery default", Model: "new model"})
	wire, _ := json.Marshal(snapshot()[0])
	var result map[string]json.RawMessage
	json.Unmarshal(wire, &result)
	if string(result["FutureSchedule"]) != `{"mode":"weekly"}` {
		t.Fatal(string(wire))
	}
	if _, e := os.Stat(configPath()); e != nil {
		t.Fatal(e)
	}
}

// Frozen v0.4.0 whole-device merge, including its v1 feature-preservation rule.
type device04 struct {
	IP, MAC, Name, Model, Serial, Power, Status, Kind, Room string
	Updated                                                 int64
	Disabled                                                bool
}
type settings04 struct {
	Version int
	Devices []device04
	Deleted map[string]int64
	Request bool
}

func read04(packet []byte, key string) settings04 {
	h := sha256.Sum256([]byte(key))
	block, _ := aes.NewCipher(h[:])
	g, _ := cipher.NewGCM(block)
	b, e := g.Open(nil, packet[:g.NonceSize()], packet[g.NonceSize():], []byte("church-device-sync-v1"))
	if e != nil {
		panic(e)
	}
	var c settings04
	if e = json.Unmarshal(b, &c); e != nil {
		panic(e)
	}
	return c
}
func merge04(local device04, c settings04) device04 {
	remote := c.Devices[0]
	r, l := remote, local
	r.Power = ""
	r.Status = ""
	l.Power = ""
	l.Status = ""
	rb, _ := json.Marshal(r)
	lb, _ := json.Marshal(l)
	if remote.Updated > local.Updated || (remote.Updated == local.Updated && string(rb) > string(lb)) {
		if c.Version < 2 {
			remote.Disabled = local.Disabled
		}
		return remote
	}
	return local
}
func TestV04BridgeConvergesInEitherDeliveryOrder(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	const key = "0123456789abcdef0123456789abcdef"
	secrets.Lock()
	previous := secrets.SyncKey
	secrets.SyncKey = key
	secrets.Unlock()
	defer func() { secrets.Lock(); secrets.SyncKey = previous; secrets.Unlock() }()
	for _, order := range [][]int{{1, 2}, {2, 1}} {
		state.Deleted = nil
		state.TVs = []TV{{IP: "192.0.2.25", Name: "Plug", Kind: "kasa", Updated: 200, Disabled: false, DisabledUpdated: 200}}
		old := device04{IP: "192.0.2.25", Name: "Plug", Kind: "kasa", Updated: 100, Disabled: true}
		packets := map[int][]byte{}
		for _, v := range []int{1, 2} {
			b, e := syncPacketVersion(true, v)
			if e != nil {
				t.Fatal(e)
			}
			packets[v] = b
		}
		for _, v := range order {
			old = merge04(old, read04(packets[v], key))
			reply, _ := json.Marshal(settings04{Version: 2, Devices: []device04{old}})
			var c sharedConfig
			json.Unmarshal(reply, &c)
			mergeConfig(c)
		}
		got := snapshot()[0]
		if old.Disabled || got.Disabled || got.Updated != 200 || got.DisabledUpdated != 200 {
			t.Fatalf("order %v: old=%+v new=%+v", order, old, got)
		}
		// Repeated legacy echoes must not inflate revisions indefinitely.
		for i := 0; i < 5; i++ {
			b, _ := syncPacketVersion(false, 2)
			old = merge04(old, read04(b, key))
			reply, _ := json.Marshal(settings04{Version: 2, Devices: []device04{old}})
			var c sharedConfig
			json.Unmarshal(reply, &c)
			mergeConfig(c)
		}
		if got := snapshot()[0]; got.Updated != 200 || got.DisabledUpdated != 200 {
			t.Fatal("legacy echo changed revision", got)
		}
	}
}
