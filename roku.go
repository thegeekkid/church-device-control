package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type TV struct {
	IP     string
	Name   string
	Model  string
	Serial string
	Power  string
	Status string
}
type deviceInfo struct {
	XMLName xml.Name `xml:"device-info"`
	Name    string   `xml:"friendly-device-name"`
	Model   string   `xml:"model-name"`
	Serial  string   `xml:"serial-number"`
	Power   string   `xml:"power-mode"`
	IsTV    string   `xml:"is-tv"`
}

var client = &http.Client{Timeout: 4 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func validIP(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && ip.To4() != nil && !ip.IsMulticast() && !ip.IsUnspecified() && !ip.Equal(net.IPv4bcast)
}
func inspect(ip string) (TV, error) {
	if !validIP(ip) {
		return TV{}, fmt.Errorf("Enter an IPv4 address, such as 192.168.1.42")
	}
	r, e := client.Get("http://" + ip + ":8060/query/device-info")
	if e != nil {
		return TV{}, e
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return TV{}, fmt.Errorf("device returned HTTP %d", r.StatusCode)
	}
	var d deviceInfo
	if e = xml.NewDecoder(io.LimitReader(r.Body, 128*1024)).Decode(&d); e != nil {
		return TV{}, e
	}
	if d.IsTV != "true" {
		return TV{}, fmt.Errorf("device is not reporting itself as a Roku TV")
	}
	name := d.Name
	if name == "" {
		name = d.Model
	}
	return TV{ip, name, d.Model, d.Serial, d.Power, "Reachable"}, nil
}
func power(ip, cmd string) error {
	if !validIP(ip) || (cmd != "PowerOn" && cmd != "PowerOff") {
		return fmt.Errorf("invalid command")
	}
	r, e := client.Post("http://"+ip+":8060/keypress/"+cmd, "application/x-www-form-urlencoded", nil)
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d — check Roku network access setting", r.StatusCode)
	}
	return nil
}
func discover() ([]string, error) {
	ifaces, e := net.Interfaces()
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	var successes int
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			ip, _, _ := net.ParseCIDR(a.String())
			if ip.To4() == nil {
				continue
			}
			c, e := net.ListenUDP("udp4", &net.UDPAddr{IP: ip})
			if e != nil {
				continue
			}
			successes++
			wg.Add(1)
			go func(c *net.UDPConn) {
				defer wg.Done()
				defer c.Close()
				c.SetDeadline(time.Now().Add(4 * time.Second))
				msg := "M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: roku:ecp\r\n\r\n"
				for n := 0; n < 2; n++ {
					c.WriteToUDP([]byte(msg), &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900})
				}
				buf := make([]byte, 8192)
				for {
					n, _, err := c.ReadFromUDP(buf)
					if err != nil {
						return
					}
					for _, line := range strings.Split(string(buf[:n]), "\n") {
						k, v, ok := strings.Cut(line, ":")
						if ok && strings.EqualFold(strings.TrimSpace(k), "location") {
							u, err := url.Parse(strings.TrimSpace(v))
							if err == nil && u.Scheme == "http" && u.Port() == "8060" && validIP(u.Hostname()) {
								mu.Lock()
								seen[u.Hostname()] = true
								mu.Unlock()
							}
						}
					}
				}
			}(c)
		}
	}
	wg.Wait()
	if successes == 0 {
		return nil, fmt.Errorf("No active IPv4 network interface")
	}
	out := []string{}
	for ip := range seen {
		out = append(out, ip)
	}
	return out, nil
}

var state struct {
	sync.Mutex
	TVs     []TV
	Message string
	Dirty   bool
	Busy    bool
}

func configPath() string {
	root, e := os.UserConfigDir()
	if e != nil {
		root = os.TempDir()
	}
	return filepath.Join(root, "ChurchRoku", "tvs.json")
}
func saveLocked() error {
	p := configPath()
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	b, e := json.MarshalIndent(state.TVs, "", "  ")
	if e != nil {
		return e
	}
	tmp := p + ".tmp"
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, p)
}
func load() {
	b, e := os.ReadFile(configPath())
	if e == nil {
		json.Unmarshal(b, &state.TVs)
	}
	filtered := []TV{}
	for _, tv := range state.TVs {
		if validIP(tv.IP) {
			tv.Status = "Saved — refresh to check"
			tv.Power = ""
			filtered = append(filtered, tv)
		}
	}
	state.TVs = filtered
	state.Message = "Discover TVs to get started. Saved TVs remain available while asleep."
	state.Dirty = true
}
func upsert(t TV) {
	state.Lock()
	defer state.Unlock()
	for i, old := range state.TVs {
		if old.IP == t.IP || (old.Serial != "" && old.Serial == t.Serial) {
			t.Name = old.Name
			state.TVs[i] = t
			state.Dirty = true
			return
		}
	}
	state.TVs = append(state.TVs, t)
	state.Dirty = true
}
func runJob(f func() string) {
	state.Lock()
	if state.Busy {
		state.Unlock()
		return
	}
	state.Busy = true
	state.Message = "Working…"
	state.Dirty = true
	state.Unlock()
	go func() {
		msg := f()
		state.Lock()
		if e := saveLocked(); e != nil {
			msg += " | Could not save: " + e.Error()
		}
		state.Message = msg
		state.Busy = false
		state.Dirty = true
		state.Unlock()
	}()
}
func snapshot() []TV { state.Lock(); defer state.Unlock(); return append([]TV(nil), state.TVs...) }
func refreshOnLaunch() {
	if len(snapshot()) > 0 {
		runJob(refresh)
	}
}
func refresh() string { return refreshTVs(snapshot()) }
func refreshTVs(tvs []TV) string {
	for _, t := range tvs {
		fresh, e := inspect(t.IP)
		if e != nil {
			state.Lock()
			for i := range state.TVs {
				if state.TVs[i].IP == t.IP {
					state.TVs[i].Status = "Unreachable (asleep or offline)"
					state.TVs[i].Power = "unknown"
				}
			}
			state.Unlock()
		} else {
			upsert(fresh)
		}
	}
	return "Refresh complete. Unreachable does not necessarily mean powered off."
}
func sendPower(tvs []TV, cmd string) string {
	ok := 0
	for _, t := range tvs {
		e := power(t.IP, cmd)
		state.Lock()
		for i := range state.TVs {
			if state.TVs[i].IP == t.IP {
				if e != nil {
					state.TVs[i].Status = "Failed: " + e.Error()
				} else {
					state.TVs[i].Status = cmd + " accepted; awaiting status check"
					ok++
				}
			}
		}
		state.Unlock()
	}
	return fmt.Sprintf("%s: %d of %d accepted. Acceptance does not confirm the screen changed.", cmd, ok, len(tvs))
}

// Keep the delay and network queries in the existing background job so the
// window continues repainting and another command cannot race verification.
func sendPowerAndRefresh(tvs []TV, cmd string) string {
	result := sendPower(tvs, cmd)
	state.Lock()
	state.Message = "Command sent. Checking TV status in one second…"
	state.Dirty = true
	state.Unlock()
	time.Sleep(time.Second)
	state.Lock()
	state.Message = "Checking TV status…"
	state.Dirty = true
	state.Unlock()
	refreshTVs(tvs)
	return strings.TrimSuffix(result, " Acceptance does not confirm the screen changed.") + " Status refreshed."
}

// The dot represents verified power state, not simply network reachability.
func tvIndicator(t TV) (int, string) {
	if t.Status != "Reachable" {
		if strings.HasPrefix(t.Status, "Failed:") {
			return 2, "Error"
		}
		if strings.Contains(t.Status, "accepted") {
			return 2, "Pending"
		}
		return 2, "Unknown"
	}
	switch strings.ToLower(t.Power) {
	case "poweron":
		return 0, "On"
	case "poweroff", "displayoff", "ready", "readysuspend", "suspend", "standby":
		return 1, "Off"
	default:
		return 2, "Unknown"
	}
}
