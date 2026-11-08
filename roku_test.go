package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type rewriteTransport struct {
	url  string
	base http.RoundTripper
}

func (r rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c := req.Clone(req.Context())
	u := *c.URL
	c.URL = &u
	c.URL.Scheme = "http"
	c.URL.Host = strings.TrimPrefix(r.url, "http://")
	return r.base.RoundTrip(c)
}
func TestRokuProtocol(t *testing.T) {
	var calls []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/query/device-info":
			io.WriteString(w, `<device-info><is-tv>true</is-tv><friendly-device-name>Lobby</friendly-device-name><serial-number>abc</serial-number><model-name>TCL</model-name><power-mode>PowerOn</power-mode></device-info>`)
		case "/keypress/PowerOn", "/keypress/PowerOff":
			w.WriteHeader(200)
		default:
			w.WriteHeader(404)
		}
	}))
	defer s.Close()
	old := client
	client = &http.Client{Transport: rewriteTransport{s.URL, http.DefaultTransport}}
	defer func() { client = old }()
	tv, e := inspect("192.168.1.2")
	if e != nil || tv.Name != "Lobby" || tv.Power != "PowerOn" {
		t.Fatalf("%+v %v", tv, e)
	}
	for _, cmd := range []string{"PowerOn", "PowerOff"} {
		if e := power(tv.IP, cmd); e != nil {
			t.Fatal(e)
		}
	}
	if e := power(tv.IP, "Power"); e == nil {
		t.Fatal("toggle allowed")
	}
	want := []string{"GET /query/device-info", "POST /keypress/PowerOn", "POST /keypress/PowerOff"}
	if strings.Join(calls, ";") != strings.Join(want, ";") {
		t.Fatal(calls)
	}
}
func TestFailures(t *testing.T) {
	for _, body := range []string{`<device-info><is-tv>false</is-tv></device-info>`, `bad xml`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) }))
		old := client
		client = &http.Client{Transport: rewriteTransport{s.URL, http.DefaultTransport}}
		if _, e := inspect("192.168.1.2"); e == nil {
			t.Fatal("accepted invalid TV")
		}
		client = old
		s.Close()
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403) }))
	defer s.Close()
	old := client
	client = &http.Client{Transport: rewriteTransport{s.URL, http.DefaultTransport}}
	defer func() { client = old }()
	if e := power("192.168.1.2", "PowerOff"); e == nil {
		t.Fatal("accepted 403")
	}
}
func TestSaveAndOfflineRetention(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	state.TVs = nil
	upsert(TV{IP: "192.168.1.5", Name: "Lobby", Serial: "abc"})
	upsert(TV{IP: "192.168.1.6", Name: "New default", Serial: "abc"})
	if len(state.TVs) != 1 || state.TVs[0].Name != "Lobby" || state.TVs[0].IP != "192.168.1.6" {
		t.Fatal(state.TVs)
	}
	state.Lock()
	e := saveLocked()
	state.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	state.TVs = nil
	load()
	if len(state.TVs) != 1 || state.TVs[0].Name != "Lobby" {
		t.Fatal(state.TVs)
	}
	if _, e := os.Stat(filepath.Clean(configPath())); e != nil {
		t.Fatal(e)
	}
}
func TestInvalidIP(t *testing.T) {
	for _, ip := range []string{"localhost", "http://example.com", "239.255.255.250", "0.0.0.0", "255.255.255.255", "::1", "192.168.1.2/path"} {
		if validIP(ip) {
			t.Fatal(ip)
		}
	}
}

func TestPowerIndicatorDoesNotGuessOfflineState(t *testing.T) {
	cases := []struct {
		tv    TV
		image int
		text  string
	}{
		{TV{Power: "PowerOn", Status: "Reachable"}, 0, "On"},
		{TV{Power: "DisplayOff", Status: "Reachable"}, 1, "Off"},
		{TV{Power: "ReadySuspend", Status: "Reachable"}, 1, "Off"},
		{TV{Power: "PowerOn", Status: "Unreachable (asleep or offline)"}, 2, "Unknown"},
		{TV{Power: "PowerOn", Status: "PowerOff accepted; refresh to verify"}, 2, "Pending"},
		{TV{Power: "PowerOff", Status: "Failed: HTTP 403"}, 2, "Error"},
		{TV{Power: "Unexpected", Status: "Reachable"}, 2, "Unknown"},
	}
	for _, c := range cases {
		image, text := tvIndicator(c.tv)
		if image != c.image || text != c.text {
			t.Errorf("%+v: got %d %s, want %d %s", c.tv, image, text, c.image, c.text)
		}
	}
}

func TestPowerCommandRefreshesAfterDelay(t *testing.T) {
	var sent, queried time.Time
	var paths []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		if r.Method == "POST" {
			sent = time.Now()
			return
		}
		queried = time.Now()
		io.WriteString(w, `<device-info><is-tv>true</is-tv><friendly-device-name>Factory name</friendly-device-name><power-mode>DisplayOff</power-mode></device-info>`)
	}))
	defer s.Close()
	old := client
	client = &http.Client{Transport: rewriteTransport{s.URL, http.DefaultTransport}}
	defer func() { client = old }()
	state.TVs = []TV{{IP: "192.168.1.5", Name: "Lobby", Power: "PowerOn", Status: "Reachable"}, {IP: "192.168.1.6", Name: "Other room", Power: "PowerOn", Status: "Reachable"}}
	result := sendPowerAndRefresh(snapshot()[:1], "PowerOff")
	if queried.Sub(sent) < time.Second {
		t.Fatalf("status was queried too soon: %v", queried.Sub(sent))
	}
	if strings.Join(paths, ";") != "POST /keypress/PowerOff;GET /query/device-info" {
		t.Fatal(paths)
	}
	tvs := snapshot()
	if tvs[0].Power != "DisplayOff" || tvs[0].Name != "Lobby" || tvs[0].Status != "Reachable" {
		t.Fatal(tvs[0])
	}
	if tvs[1].Power != "PowerOn" {
		t.Fatal("uncommanded TV changed")
	}
	if !strings.Contains(result, "1 of 1 accepted") || !strings.Contains(result, "Status refreshed") {
		t.Fatal(result)
	}
}
func TestStartupRefreshUsesSavedTVs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/query/device-info" {
			t.Errorf("unexpected startup request: %s %s", r.Method, r.URL.Path)
		}
		io.WriteString(w, `<device-info><is-tv>true</is-tv><friendly-device-name>Factory name</friendly-device-name><power-mode>PowerOn</power-mode></device-info>`)
	}))
	defer s.Close()
	old := client
	client = &http.Client{Transport: rewriteTransport{s.URL, http.DefaultTransport}}
	defer func() { client = old }()
	state.Lock()
	state.TVs = []TV{{IP: "192.168.1.5", Name: "Lobby"}}
	state.Busy = false
	e := saveLocked()
	state.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	load()
	refreshOnLaunch()
	deadline := time.Now().Add(5 * time.Second)
	for {
		state.Lock()
		busy := state.Busy
		state.Unlock()
		if !busy {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("startup refresh did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
	tvs := snapshot()
	if len(tvs) != 1 || tvs[0].Name != "Lobby" || tvs[0].Power != "PowerOn" || tvs[0].Status != "Reachable" {
		t.Fatal(tvs)
	}
}
