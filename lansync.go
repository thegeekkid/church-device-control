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
	Version int
	Devices []TV
	Deleted map[string]int64
	Request bool
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
func sealConfig(c sharedConfig, key string) ([]byte, error) {
	if len(key) < 24 {
		return nil, fmt.Errorf("sync key must contain at least 24 characters")
	}
	hash := sha256.Sum256([]byte(key))
	block, _ := aes.NewCipher(hash[:])
	g, _ := cipher.NewGCM(block)
	nonce := make([]byte, g.NonceSize())
	if _, e := rand.Read(nonce); e != nil {
		return nil, e
	}
	b, e := json.Marshal(c)
	if e != nil {
		return nil, e
	}
	if len(b) > 55000 {
		return nil, fmt.Errorf("settings exceed LAN packet limit")
	}
	return g.Seal(nonce, nonce, b, []byte("church-device-sync-v1")), nil
}
func openConfig(b []byte, key string) (sharedConfig, error) {
	var c sharedConfig
	if len(key) < 24 {
		return c, fmt.Errorf("sync disabled")
	}
	hash := sha256.Sum256([]byte(key))
	block, _ := aes.NewCipher(hash[:])
	g, _ := cipher.NewGCM(block)
	if len(b) < g.NonceSize() {
		return c, fmt.Errorf("invalid packet")
	}
	plain, e := g.Open(nil, b[:g.NonceSize()], b[g.NonceSize():], []byte("church-device-sync-v1"))
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(plain, &c); e != nil {
		return c, e
	}
	if (c.Version != 1 && c.Version != 2) || len(c.Devices) > 500 || len(c.Deleted) > 2000 {
		return c, fmt.Errorf("unsupported settings")
	}
	for _, t := range c.Devices {
		if !validIP(t.IP) || (t.Kind != "" && t.Kind != "roku" && t.Kind != "kasa") || len(t.Name) > 200 || len(t.Room) > 200 || t.Updated > time.Now().Add(5*time.Minute).UnixNano() {
			return c, fmt.Errorf("invalid device settings")
		}
	}
	return c, nil
}
func mergeConfig(c sharedConfig) bool {
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
			if remote.Updated > local.Updated || (remote.Updated == local.Updated && string(rb) > string(lb)) {
				if remote.MAC == "" {
					remote.MAC = local.MAC
				}
				if c.Version < 2 {
					remote.Disabled = local.Disabled
				}
				remote.Power = local.Power
				remote.Status = local.Status
				state.TVs[i] = remote
				changed = true
			}
			break
		}
		if !found {
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
func syncPacket(request bool) ([]byte, error) {
	state.Lock()
	c := configSnapshotLocked(true)
	state.Unlock()
	c.Request = request
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
		for {
			n, addr, e := c.ReadFromUDP(buf)
			if e != nil {
				return
			}
			cfg, e := openConfig(buf[:n], syncKey())
			if e != nil {
				continue
			}
			mergeConfig(cfg)
			if cfg.Request {
				if b, e := syncPacket(false); e == nil {
					c.WriteToUDP(b, addr)
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
	b, e := syncPacket(true)
	if e != nil {
		return e.Error()
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
	sent := false
	for ip := range destinations {
		if _, e = lanConn.WriteToUDP(b, &net.UDPAddr{IP: net.ParseIP(ip), Port: syncPort}); e == nil {
			sent = true
		}
	}
	if !sent {
		return "LAN sync broadcast failed; check the network and firewall."
	}
	return "LAN sync sent at " + time.Now().Format("15:04:05")
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
