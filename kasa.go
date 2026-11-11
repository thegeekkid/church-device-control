package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pborges/tapo"
	"net"
	"strings"
	"time"
)

func kasaClient(ip string) (*tapo.Strip, error) {
	if !validIP(ip) {
		return nil, fmt.Errorf("Enter a valid IPv4 address")
	}
	secrets.Lock()
	u, p := secrets.User, secrets.Password
	secrets.Unlock()
	return tapo.New(ip, tapo.WithCredentials(u, p), tapo.WithTimeout(3*time.Second))
}

var connectKasa = kasaClient

func inspectDevice(t TV) (TV, error) {
	if t.Kind == "kasa" {
		return inspectKasa(t.IP)
	}
	return inspect(t.IP)
}
func inspectKasa(ip string) (TV, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c, raw, e := probeKasa(ctx, ip)
	if e != nil {
		return TV{}, e
	}
	if e == nil {
		var r struct {
			System struct {
				Info struct {
					Alias, Model, DeviceID string
					Relay                  *int `json:"relay_state"`
					Code                   int  `json:"err_code"`
				} `json:"get_sysinfo"`
			}
		}
		if decodeKasa(raw, &r) == nil && r.System.Info.Relay != nil && r.System.Info.Code == 0 {
			d := r.System.Info
			p := "PowerOff"
			if *d.Relay != 0 {
				p = "PowerOn"
			}
			return TV{IP: ip, Name: d.Alias, Model: d.Model, Serial: d.DeviceID, Kind: "kasa", Power: p, Status: "Reachable"}, nil
		}
	}
	info, e := c.Info(ctx)
	if e != nil {
		return TV{}, fmt.Errorf("Kasa authentication/connection failed; check credentials and model compatibility: %w", e)
	}
	outlets, e := c.Outlets(ctx)
	if e != nil {
		return TV{}, e
	}
	if len(outlets) != 1 {
		return TV{}, fmt.Errorf("this release supports single-outlet plugs; found %d outlets", len(outlets))
	}
	p := "PowerOff"
	if outlets[0].On {
		p = "PowerOn"
	}
	return TV{IP: ip, Name: info.Alias, Model: info.Model, Serial: info.DeviceID, Kind: "kasa", Power: p, Status: "Reachable"}, nil
}
func devicePower(t TV, cmd string) error {
	if t.Kind != "kasa" {
		return power(t.IP, cmd)
	}
	if cmd != "PowerOn" && cmd != "PowerOff" {
		return fmt.Errorf("invalid command")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c, raw, e := probeKasa(ctx, t.IP)
	if e != nil {
		return e
	}
	if e == nil {
		var r struct {
			System struct {
				Info struct {
					Relay *int `json:"relay_state"`
				} `json:"get_sysinfo"`
			}
		}
		if decodeKasa(raw, &r) == nil && r.System.Info.Relay != nil {
			v := 0
			if cmd == "PowerOn" {
				v = 1
			}
			response, e := c.Query(ctx, map[string]any{"system": map[string]any{"set_relay_state": map[string]int{"state": v}}})
			if e != nil {
				return e
			}
			var result struct {
				System struct {
					Set struct {
						Code *int `json:"err_code"`
					} `json:"set_relay_state"`
				}
			}
			if decodeKasa(response, &result) != nil || result.System.Set.Code == nil || *result.System.Set.Code != 0 {
				return fmt.Errorf("Kasa rejected relay command")
			}
			return nil
		}
	}
	outlets, e := c.Outlets(ctx)
	if e != nil {
		return e
	}
	if len(outlets) != 1 {
		return fmt.Errorf("single-outlet plugs only")
	}
	return c.SetOutlet(ctx, outlets[0].ID, cmd == "PowerOn")
}
func discoverKasa() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ds, e := tapo.Discover(ctx, 3*time.Second)
	if e != nil {
		return "Kasa discovery: " + e.Error()
	}
	n := 0
	for _, ip := range legacyKasaIPs() {
		ds = append(ds, tapo.DiscoveredDevice{Host: ip})
	}
	seen := map[string]bool{}
	for _, d := range ds {
		if seen[d.Host] {
			continue
		}
		seen[d.Host] = true
		if validIP(d.Host) {
			t, e := inspectKasa(d.Host)
			if e == nil {
				upsert(t)
				n++
			} else {
				upsert(TV{IP: d.Host, Name: d.Device.Alias, Model: d.Device.Model, Kind: "kasa", Status: "Failed: " + e.Error()})
			}
		}
	}
	return fmt.Sprintf("Kasa: %d plugs authenticated. Manual IP entry is also available.", n)
}

func decodeKasa(v any, out any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, out)
}

// Older Kasa plugs advertise on UDP 9999 rather than UDP 20002.
func legacyKasaIPs() []string {
	c, e := net.ListenUDP("udp4", &net.UDPAddr{})
	if e != nil {
		return nil
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(2 * time.Second))
	query := []byte(`{"system":{"get_sysinfo":{}}}`)
	key := byte(171)
	for i := range query {
		query[i] ^= key
		key = query[i]
	}
	c.WriteToUDP(query, &net.UDPAddr{IP: net.IPv4bcast, Port: 9999})
	seen := map[string]bool{}
	buf := make([]byte, 16000)
	for {
		n, addr, e := c.ReadFromUDP(buf)
		if e != nil {
			break
		}
		plain := append([]byte(nil), buf[:n]...)
		key = 171
		for i, b := range plain {
			plain[i] = b ^ key
			key = b
		}
		var r map[string]any
		if json.Unmarshal(plain, &r) == nil && r["system"] != nil {
			seen[addr.IP.String()] = true
		}
	}
	out := []string{}
	for ip := range seen {
		out = append(out, ip)
	}
	return out
}

// Account credentials remain the first choice. Newer Kasa IOT firmware may
// instead use the account e-mail + kasaSetup for the local KLAP handshake.
// Reference: python-kasa/python-kasa#1840. This never changes cloud credentials.
var setupKasaClient = func(ip, user string) (*tapo.Strip, error) {
	return tapo.New(ip, tapo.WithCredentials(user, "kasaSetup"), tapo.WithTimeout(3*time.Second), tapo.WithDisableLegacy())
}

func probeKasa(ctx context.Context, ip string) (*tapo.Strip, tapo.RawResponse, error) {
	c, e := connectKasa(ip)
	if e != nil {
		return nil, nil, e
	}
	request := map[string]any{"system": map[string]any{"get_sysinfo": nil}}
	r, e := c.Query(ctx, request)
	if e == nil {
		return c, r, nil
	}
	var auth *tapo.AuthenticationError
	if !errors.As(e, &auth) {
		return nil, nil, fmt.Errorf("Kasa connection/protocol error: %w", e)
	}
	secrets.Lock()
	user := secrets.User
	secrets.Unlock()
	candidates := []string{user, strings.ToLower(strings.TrimSpace(user))}
	seen := map[string]bool{}
	for _, u := range candidates {
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		candidate, ce := setupKasaClient(ip, u)
		if ce != nil {
			return nil, nil, ce
		}
		response, ce := candidate.Query(ctx, request)
		if ce == nil {
			return candidate, response, nil
		}
		if !errors.As(ce, &auth) {
			return nil, nil, fmt.Errorf("Kasa setup-mode connection/protocol error: %w", ce)
		}
	}
	return nil, nil, fmt.Errorf("Kasa KLAP authentication mismatch after account and setup modes; model/firmware or device account binding may be incompatible")
}
