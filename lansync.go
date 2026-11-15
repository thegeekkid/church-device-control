package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

const syncPort = 45439

// Settings only: no power state, commands or account credentials cross the LAN.
type sharedConfig struct {
	Version      int
	Capabilities []string `json:",omitempty"`
	Devices      []TV
	Deleted      map[string]int64
	Request      bool
	// Optional authenticated send time; old readers ignore it. Local snapshots
	// omit it, so it never changes device revisions or saved settings.
	SentAt int64 `json:",omitempty"`
}

func deviceID(t TV) string {
	k := t.Kind
	if k == "" {
		k = "roku"
	}
	if t.Serial != "" {
		return k + ":" + t.Serial
	}
	return k + ":ip:" + t.IP
}
func configSnapshotLocked(wire bool) sharedConfig {
	c := sharedConfig{Version: 2, Deleted: map[string]int64{}}
	for k, v := range state.Deleted {
		c.Deleted[k] = v
	}
	for _, t := range state.TVs {
		if wire {
			t.Power = ""
			t.Status = ""
		}
		c.Devices = append(c.Devices, t)
	}
	return c
}
func syncCipher(key string) (cipher.AEAD, error) {
	if len(key) < 24 {
		return nil, fmt.Errorf("sync key must contain at least 24 characters")
	}
	// This derivation is the deployed v0.3/v0.4 wire contract. Generate key
	// supplies random bytes, not a human password; changing the derivation here
	// would disconnect existing peers. AES-GCM below authenticates each packet.
	// This is not password stretching: manually chosen weak phrases remain
	// guessable. Prefer the CSPRNG-based Settings generator (sync_security.go).
	hash := sha256.Sum256([]byte(key))
	block, e := aes.NewCipher(hash[:])
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(block)
}
func sealConfig(c sharedConfig, key string) ([]byte, error) {
	g, e := syncCipher(key)
	if e != nil {
		return nil, e
	}
	nonce := make([]byte, g.NonceSize())
	if _, e := rand.Read(nonce); e != nil {
		return nil, e
	}
	b, e := json.Marshal(c)
	if e != nil {
		return nil, e
	}
	if len(b) > maxSyncPlaintext {
		return nil, fmt.Errorf("settings exceed LAN packet limit")
	}
	return g.Seal(nonce, nonce, b, []byte("church-device-sync-v1")), nil
}
func openConfig(b []byte, key string) (sharedConfig, error) {
	var c sharedConfig
	if len(key) < 24 {
		return c, fmt.Errorf("sync disabled")
	}
	g, e := syncCipher(key)
	if e != nil {
		return c, e
	}
	if len(b) < g.NonceSize()+g.Overhead() || len(b) > maxSyncPlaintext+g.NonceSize()+g.Overhead() {
		return c, fmt.Errorf("invalid packet")
	}
	// Open verifies the GCM authentication tag before parsing or trusting JSON.
	// Only holders of the shared key can create a valid packet. This is group
	// authentication, not individual-PC authorization or replay protection.
	plain, e := g.Open(nil, b[:g.NonceSize()], b[g.NonceSize():], []byte("church-device-sync-v1"))
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(plain, &c); e != nil {
		return c, e
	}
	if e := normalizeV04Bridge(&c); e != nil {
		return c, e
	}
	now := time.Now()
	if c.SentAt != 0 && (c.SentAt < now.Add(-syncClockWindow).UnixNano() || c.SentAt > now.Add(syncClockWindow).UnixNano()) {
		return c, fmt.Errorf("expired sync packet; check PC clocks")
	}
	if e := validateSharedConfig(c, now); e != nil {
		return c, e
	}
	return c, nil
}
func mergeConfig(c sharedConfig) bool {
	// openConfig checks wire input first. Repeat settings validation here so a
	// direct caller cannot bypass the trust boundary or partially merge bad data.
	if validateSharedConfig(c, time.Now()) != nil {
		return false
	}
	state.Lock()
	defer state.Unlock()
	if state.Deleted == nil {
		state.Deleted = map[string]int64{}
	}
	changed := false
	for id, ts := range c.Deleted {
		if ts > state.Deleted[id] && ts <= time.Now().Add(5*time.Minute).UnixNano() {
			state.Deleted[id] = ts
			changed = true
		}
	}
	for _, remote := range c.Devices {
		remote.Power = ""
		remote.Status = "Saved — refresh to check"
		id := deviceID(remote)
		if state.Deleted[id] >= remote.Updated && state.Deleted[id] > 0 {
			continue
		}
		found := false
		for i, local := range state.TVs {
			if deviceID(local) != id && !(local.IP == remote.IP && deviceKindKey(local) == deviceKindKey(remote)) {
				continue
			}
			found = true
			l, r := local, remote
			l.Power = ""
			l.Status = ""
			r.Status = ""
			lb, _ := json.Marshal(l)
			rb, _ := json.Marshal(r)
			remoteWins := remote.Updated > local.Updated || (remote.Updated == local.Updated && string(rb) > string(lb))
			next := local
			if remoteWins {
				next = remote
				if next.MAC == "" {
					next.MAC = local.MAC
				}
				next.Power = local.Power
				next.Status = local.Status
			}
			next.Extras = mergeExtras(local.Extras, remote.Extras, remoteWins)
			if remoteWins && len(c.Capabilities) == 0 && sameCommonSettings(local, next) && sameExtras(local.Extras, next.Extras) {
				next.Updated = local.Updated
			}
			// New feature values have their own revision. Legacy edits to shared
			// fields must not reset values absent from their protocol capabilities.
			next.Disabled = local.Disabled
			next.DisabledUpdated = disabledRevision(local)
			if supportsDisabled(c) {
				rr, lr := disabledRevision(remote), disabledRevision(local)
				explicit := remote.DisabledUpdated > 0
				if (explicit || remote.Disabled != local.Disabled) && (rr > lr || (explicit && rr == lr && remote.Disabled && !local.Disabled)) {
					next.Disabled = remote.Disabled
					next.DisabledUpdated = rr
				}
			}
			before, _ := json.Marshal(local)
			after, _ := json.Marshal(next)
			if string(before) != string(after) {
				state.TVs[i] = next
				changed = true
			}

			break
		}
		if !found {
			if !supportsDisabled(c) {
				remote.Disabled = false
				remote.DisabledUpdated = 0
			}
			state.TVs = append(state.TVs, remote)
			changed = true
		}
	}
	filtered := state.TVs[:0]
	for _, t := range state.TVs {
		if ts := state.Deleted[deviceID(t)]; ts > 0 && ts >= t.Updated {
			changed = true
			continue
		}
		filtered = append(filtered, t)
	}
	state.TVs = filtered
	if changed {
		state.Dirty = true
		state.SyncStatus = "LAN settings received at " + time.Now().Format("15:04:05")
		if e := saveLocked(); e != nil {
			state.SyncStatus = "Sync could not save: " + e.Error()
		}
	}
	return changed
}
func syncKey() string { secrets.Lock(); defer secrets.Unlock(); return secrets.SyncKey }

// Version 1 is the stable additive envelope. Version 2 is also emitted to
// bridge v0.4.0, which required it to understand Disabled.
func supportsDisabled(c sharedConfig) bool {
	if c.Version == 2 {
		return true
	}
	for _, feature := range c.Capabilities {
		if feature == "disabled-v1" {
			return true
		}
	}
	return false
}
func syncPacket(request bool) ([]byte, error) { return syncPacketVersion(request, 1) }
func syncPacketVersion(request bool, version int) ([]byte, error) {
	state.Lock()
	c := configSnapshotLocked(true)
	state.Unlock()
	c.Version = version
	c.Capabilities = []string{"disabled-v1", "unknown-fields-v1"}
	c.Request = request
	c.SentAt = time.Now().UnixNano()
	for i := range c.Devices {
		c.Devices[i].DisabledUpdated = disabledRevision(c.Devices[i])
	}
	if version == 2 {
		prepareV04Bridge(&c)
	}
	return sealConfig(c, syncKey())
}

func syncStatus(s string) { state.Lock(); state.SyncStatus = s; state.Dirty = true; state.Unlock() }

var lanConn *net.UDPConn

func startLAN() {
	c, e := net.ListenUDP("udp4", &net.UDPAddr{Port: syncPort})
	if e != nil {
		syncStatus("LAN sync unavailable: " + e.Error())
		return
	}
	lanConn = c
	go func() {
		buf := make([]byte, 60000)
		var receiver syncReceiver
		for {
			n, addr, e := c.ReadFromUDP(buf)
			if e != nil {
				return
			}
			// The receiver authenticates/validates, then suppresses duplicate
			// datagrams before merging or replying to a request.
			cfg, e := receiver.open(buf[:n], syncKey())
			if e != nil {
				continue
			}
			mergeConfig(cfg)
			if cfg.Request {
				if b, e := syncPacketVersion(false, cfg.Version); e == nil {
					if e := writeSyncPacket(c.WriteToUDP, b, addr); e != nil {
						syncStatus(fmt.Sprintf("LAN sync reply to %s failed: %v", addr, e))
					}
				} else {
					syncStatus("LAN sync reply could not be prepared: " + e.Error())
				}
			}
		}
	}()
}
func syncNow() string {
	if syncKey() == "" {
		return "LAN sync disabled; configure the same sync key on each PC."
	}
	if lanConn == nil {
		return "LAN sync unavailable; check whether another instance is open."
	}
	var packets [][]byte
	for _, version := range []int{2, 1} {
		b, e := syncPacketVersion(true, version)
		if e != nil {
			return e.Error()
		}
		packets = append(packets, b)
	}
	destinations := map[string]bool{"255.255.255.255": true}
	ifs, _ := net.Interfaces()
	for _, f := range ifs {
		if f.Flags&net.FlagUp == 0 || f.Flags&net.FlagLoopback != 0 {
			continue
		}
		as, _ := f.Addrs()
		for _, a := range as {
			ip, n, e := net.ParseCIDR(a.String())
			if e != nil || ip.To4() == nil {
				continue
			}
			v := ip.To4()
			broadcast := net.IPv4(v[0]|^n.Mask[0], v[1]|^n.Mask[1], v[2]|^n.Mask[2], v[3]|^n.Mask[3])
			destinations[broadcast.String()] = true
		}
	}
	return broadcastSyncPackets(packets, destinations, lanConn.WriteToUDP)
}
func periodicWork() string { s := syncNow(); syncStatus(s); return refresh() }
func roomTargets(room string) []TV {
	out := []TV{}
	for _, t := range snapshot() {
		if strings.EqualFold(strings.TrimSpace(t.Room), strings.TrimSpace(room)) && strings.TrimSpace(room) != "" && !t.Disabled {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind != "kasa" && out[j].Kind == "kasa" })
	return out
}

func deviceKindKey(t TV) string {
	if t.Kind == "kasa" {
		return "kasa"
	}
	return "roku"
}
