package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	maxSyncPlaintext         = 55000
	syncClockWindow          = 5 * time.Minute
	syncReplayWindow         = 2 * syncClockWindow
	maxRememberedSyncPackets = 4096
)

func generateSyncKey() (string, error) {
	// A random 256-bit shared key avoids the password-guessing concern of the
	// legacy SHA-256 wire derivation without changing existing keys or peers.
	var key [32]byte
	defer clear(key[:])
	if _, e := rand.Read(key[:]); e != nil {
		return "", e
	}
	return hex.EncodeToString(key[:]), nil
}

func validateDeviceMetadata(t TV) error {
	for _, field := range []struct {
		value string
		limit int
	}{
		{t.Name, 200}, {t.Room, 200}, {t.Model, 256}, {t.Serial, 256}, {t.MAC, 128},
	} {
		if len(field.value) > field.limit || strings.ContainsRune(field.value, '\x00') {
			return fmt.Errorf("invalid device metadata")
		}
	}
	return nil
}

func validateSharedConfig(c sharedConfig, now time.Time) error {
	if (c.Version != 1 && c.Version != 2) || len(c.Devices) > 500 || len(c.Deleted) > 2000 {
		return fmt.Errorf("unsupported settings")
	}
	latest := now.Add(syncClockWindow).UnixNano()
	for _, t := range c.Devices {
		if !validIP(t.IP) || (t.Kind != "" && t.Kind != "roku" && t.Kind != "kasa") || t.Updated < 0 || t.Updated > latest || t.DisabledUpdated < 0 || t.DisabledUpdated > latest {
			return fmt.Errorf("invalid device settings")
		}
		if e := validateDeviceMetadata(t); e != nil {
			return e
		}
	}
	for id, ts := range c.Deleted {
		if id == "" || len(id) > 512 || strings.ContainsRune(id, '\x00') || ts < 0 || ts > latest {
			return fmt.Errorf("invalid deletion settings")
		}
	}
	return nil
}

// Used only by the single LAN receive goroutine. The bounded cache suppresses
// exact replays (including legacy requests) during this process's lifetime.
// New peers also stamp packets so old captures expire; old peers have no send
// timestamp, so full replay expiry cannot be retrofitted to their messages.
type syncReceiver struct{ seen map[[32]byte]time.Time }

func (r *syncReceiver) open(packet []byte, key string) (sharedConfig, error) {
	c, e := openConfig(packet, key)
	if e != nil {
		return c, e
	}
	now := time.Now()
	digest := sha256.Sum256(packet)
	if received, ok := r.seen[digest]; ok && now.Sub(received) < syncReplayWindow {
		return sharedConfig{}, fmt.Errorf("duplicate sync packet")
	}
	if r.seen == nil {
		r.seen = make(map[[32]byte]time.Time)
	}
	if len(r.seen) >= maxRememberedSyncPackets {
		var oldest [32]byte
		oldestTime := now
		for id, at := range r.seen {
			if now.Sub(at) >= syncReplayWindow {
				delete(r.seen, id)
				continue
			}
			if at.Before(oldestTime) {
				oldest, oldestTime = id, at
			}
		}
		if len(r.seen) >= maxRememberedSyncPackets {
			delete(r.seen, oldest)
		}
	}
	// Never remember unauthenticated/invalid traffic: it cannot poison the cache.
	r.seen[digest] = now
	return c, nil
}
