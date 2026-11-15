package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type testRoundTrip func(*http.Request) (*http.Response, error)

func (f testRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestDisabledDevicesSkipRefreshAndCommandsButDiscoveryUpdates(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	state.TVs = []TV{{IP: "192.0.2.1", Name: "Active TV", Room: "Lobby"}, {IP: "192.0.2.2", Name: "Occasional TV", Room: "Lobby", Disabled: true}, {IP: "192.0.2.3", Name: "Occasional plug", Room: "Lobby", Kind: "kasa", Disabled: true}}
	state.Deleted = nil
	var mu sync.Mutex
	var calls []string
	old := client
	client = &http.Client{Transport: testRoundTrip(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Host)
		mu.Unlock()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`<device-info><is-tv>true</is-tv><friendly-device-name>Discovered name</friendly-device-name><model-name>Updated model</model-name><power-mode>PowerOn</power-mode></device-info>`)), Header: make(http.Header)}, nil
	})}
	defer func() { client = old }()
	before := snapshot()
	refreshTVs(before)
	sendPower(before, "PowerOn")
	for _, tv := range before[1:] {
		if e := devicePower(tv, "PowerOff"); e == nil {
			t.Fatal("direct disabled command accepted")
		}
		image, text := tvIndicator(tv)
		if image != 3 || text != "Disabled" {
			t.Fatal(image, text)
		}
	}
	mu.Lock()
	got := append([]string(nil), calls...)
	mu.Unlock()
	if strings.Join(got, ";") != "GET 192.0.2.1:8060;POST 192.0.2.1:8060" {
		t.Fatal(got)
	}
	if len(roomTargets("Lobby")) != 1 {
		t.Fatal("disabled room member included")
	}
	discovered, e := inspect("192.0.2.2")
	if e != nil {
		t.Fatal(e)
	}
	upsert(discovered)
	tv := snapshot()[1]
	if !tv.Disabled || tv.Model != "Updated model" || tv.Name != "Occasional TV" {
		t.Fatal(tv)
	}
	state.Lock()
	e = saveLocked()
	state.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	load()
	if !snapshot()[1].Disabled {
		t.Fatal("disabled flag lost after restart")
	}
	if e = setDeviceDisabled(deviceID(snapshot()[1]), false); e != nil {
		t.Fatal(e)
	}
	if deviceDisabled(snapshot()[1]) {
		t.Fatal("enable failed")
	}
}
func TestDisabledSettingSyncsAndOldPeersCannotEnableIt(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	state.TVs = []TV{{IP: "192.0.2.4", Name: "Plug", Kind: "kasa", Disabled: true, Updated: 10}}
	state.Deleted = nil
	mergeConfig(sharedConfig{Version: 1, Devices: []TV{{IP: "192.0.2.4", Name: "Older peer edit", Kind: "kasa", Updated: 20}}})
	if !snapshot()[0].Disabled {
		t.Fatal("old peer enabled a disabled device")
	}
	mergeConfig(sharedConfig{Version: 2, Devices: []TV{{IP: "192.0.2.4", Name: "New peer edit", Kind: "kasa", Disabled: false, Updated: 30}}})
	if snapshot()[0].Disabled {
		t.Fatal("new enable edit not applied")
	}
	if e := setDeviceDisabled(deviceID(snapshot()[0]), true); e != nil {
		t.Fatal(e)
	}
	state.Lock()
	wire := configSnapshotLocked(true)
	state.Unlock()
	if wire.Version != 2 || !wire.Devices[0].Disabled {
		t.Fatal(wire)
	}
}
func TestRefreshProcessesOtherDevicesDuringSlowRead(t *testing.T) {
	state.TVs = []TV{{IP: "192.0.2.10", Name: "Slow"}, {IP: "192.0.2.11", Name: "Fast"}}
	state.Deleted = nil
	slowEntered, releaseSlow, fastDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	old := client
	client = &http.Client{Transport: testRoundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.HasPrefix(r.URL.Host, "192.0.2.10:") {
			close(slowEntered)
			select {
			case <-releaseSlow:
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
		} else {
			close(fastDone)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`<device-info><is-tv>true</is-tv><power-mode>PowerOn</power-mode></device-info>`)), Header: make(http.Header)}, nil
	})}
	defer func() { client = old }()
	done := make(chan struct{})
	go func() { refreshTVs(snapshot()); close(done) }()
	defer func() { close(releaseSlow); <-done }()
	select {
	case <-slowEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("slow read never started")
	}
	select {
	case <-fastDone:
	case <-time.After(2 * time.Second):
		t.Fatal("slow device blocked fast device")
	}
}
func TestKasaReadHonorsParentDeadline(t *testing.T) {
	old := connectKasa
	defer func() { connectKasa = old }()
	// A canceled parent must propagate through the complete Kasa probe.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e := inspectKasaContext(ctx, "127.0.0.1")
	if e == nil || !strings.Contains(e.Error(), context.Canceled.Error()) {
		t.Fatal(fmt.Sprint(e))
	}
}
