package main

import (
	"encoding/binary"
	"strings"
	"testing"
	"time"
)

func TestSyncFreshnessReplayAndLegacyInterop(t *testing.T) {
	key, e := generateSyncKey()
	if e != nil {
		t.Fatal(e)
	}
	if len(key) != 64 {
		t.Fatal("generator did not produce a 256-bit hexadecimal key")
	}
	now := time.Now()
	c := sharedConfig{Version: 1, Request: true, SentAt: now.UnixNano(), Devices: []TV{{IP: "192.0.2.30", Name: "Lobby", Updated: 1}}}
	packet, e := sealConfig(c, key)
	if e != nil {
		t.Fatal(e)
	}
	old, e := legacyRead03(packet, key)
	if e != nil || len(old.Devices) != 1 || old.Devices[0].Name != "Lobby" {
		t.Fatalf("generated key/new fields broke legacy reader: %+v %v", old, e)
	}
	var receiver syncReceiver
	if _, e := receiver.open(packet, key); e != nil {
		t.Fatal(e)
	}
	if _, e := receiver.open(packet, key); e == nil {
		t.Fatal("replayed request accepted")
	}
	// A legitimate new exchange has a fresh nonce, even without settings edits.
	fresh, e := sealConfig(c, key)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := receiver.open(fresh, key); e != nil {
		t.Fatal(e)
	}
	for _, sentAt := range []int64{now.Add(-10 * time.Minute).UnixNano(), now.Add(10 * time.Minute).UnixNano(), -1} {
		c.SentAt = sentAt
		b, e := sealConfig(c, key)
		if e != nil {
			t.Fatal(e)
		}
		if _, e := receiver.open(b, key); e == nil {
			t.Fatalf("invalid send time accepted: %d", sentAt)
		}
	}
	c.SentAt = 0 // Frozen old senders have no packet timestamp.
	legacy, e := sealConfig(c, key)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := receiver.open(legacy, key); e != nil {
		t.Fatal("legacy packet rejected", e)
	}
	if _, e := receiver.open(legacy, key); e == nil {
		t.Fatal("legacy replay accepted")
	}
	bad := append([]byte(nil), packet...)
	bad[len(bad)-1] ^= 1
	var clean syncReceiver
	if _, e := clean.open(bad, key); e == nil {
		t.Fatal("tampered packet accepted")
	}
	if len(clean.seen) != 0 {
		t.Fatal("bad packet poisoned replay cache")
	}
	if _, e := clean.open(packet, key); e != nil {
		t.Fatal(e)
	}
}

func TestSyncReplayCacheStaysBounded(t *testing.T) {
	key, e := generateSyncKey()
	if e != nil {
		t.Fatal(e)
	}
	packet, e := sealConfig(sharedConfig{Version: 1}, key)
	if e != nil {
		t.Fatal(e)
	}
	for _, age := range []time.Duration{time.Minute, 11 * time.Minute} {
		r := syncReceiver{seen: map[[32]byte]time.Time{}}
		for i := 0; i < maxRememberedSyncPackets; i++ {
			var digest [32]byte
			binary.BigEndian.PutUint64(digest[:], uint64(i))
			r.seen[digest] = time.Now().Add(-age)
		}
		if _, e := r.open(packet, key); e != nil {
			t.Fatal(e)
		}
		if len(r.seen) > maxRememberedSyncPackets {
			t.Fatal("unbounded replay cache", len(r.seen))
		}
	}
}

func TestSyncRejectsInvalidSettingsBeforeAnyMerge(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	key, e := generateSyncKey()
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*sharedConfig)
	}{
		{"kind", func(c *sharedConfig) { c.Devices[0].Kind = "shell" }},
		{"metadata", func(c *sharedConfig) { c.Devices[0].Serial = strings.Repeat("a", 257) }},
		{"nul", func(c *sharedConfig) { c.Devices[0].Name = "Lobby\x00hidden" }},
		{"negative revision", func(c *sharedConfig) { c.Devices[0].Updated = -1 }},
		{"future feature", func(c *sharedConfig) { c.Devices[0].DisabledUpdated = time.Now().Add(time.Hour).UnixNano() }},
		{"bad deletion", func(c *sharedConfig) { c.Deleted = map[string]int64{"roku:other": -1} }},
		{"future deletion", func(c *sharedConfig) {
			c.Deleted = map[string]int64{"roku:other": time.Now().Add(time.Hour).UnixNano()}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state.TVs = []TV{{IP: "192.0.2.30", Name: "Original", Updated: 1}}
			state.Deleted = nil
			c := sharedConfig{Version: 1, Devices: []TV{{IP: "192.0.2.30", Name: "Changed", Updated: 2}}}
			tc.mutate(&c)
			packet, e := sealConfig(c, key)
			if e != nil {
				t.Fatal(e)
			}
			if _, e := openConfig(packet, key); e == nil {
				t.Fatal("invalid settings accepted over wire")
			}
			if mergeConfig(c) || snapshot()[0].Name != "Original" || len(state.Deleted) != 0 {
				t.Fatal("direct merge bypassed validation")
			}
		})
	}
	if _, e := openConfig(make([]byte, 60000), key); e == nil {
		t.Fatal("oversized datagram accepted")
	}
}
