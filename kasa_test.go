package main

import (
	"encoding/binary"
	"encoding/json"
	"github.com/pborges/tapo"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
)

func TestLegacyKasaPowerAndStatus(t *testing.T) {
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	var mu sync.Mutex
	relay := 1
	go func() {
		for {
			c, e := ln.Accept()
			if e != nil {
				return
			}
			func() {
				defer c.Close()
				var size uint32
				if binary.Read(c, binary.BigEndian, &size) != nil || size > 4096 {
					return
				}
				b := make([]byte, size)
				if _, e := io.ReadFull(c, b); e != nil {
					return
				}
				key := byte(171)
				for i, v := range b {
					b[i] = v ^ key
					key = v
				}
				var req map[string]map[string]json.RawMessage
				json.Unmarshal(b, &req)
				mu.Lock()
				defer mu.Unlock()
				var response any
				if raw, ok := req["system"]["set_relay_state"]; ok {
					var r struct{ State int }
					json.Unmarshal(raw, &r)
					relay = r.State
					response = map[string]any{"system": map[string]any{"set_relay_state": map[string]int{"err_code": 0}}}
				} else {
					response = map[string]any{"system": map[string]any{"get_sysinfo": map[string]any{"alias": "Test plug", "model": "HS103", "deviceId": "test-device", "mac": "11-22-33-44-55-66", "relay_state": relay, "err_code": 0}}}
				}
				out, _ := json.Marshal(response)
				key = 171
				for i := range out {
					out[i] ^= key
					key = out[i]
				}
				binary.Write(c, binary.BigEndian, uint32(len(out)))
				c.Write(out)
			}()
		}
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	n, _ := strconv.Atoi(port)
	old := connectKasa
	connectKasa = func(ip string) (*tapo.Strip, error) { return tapo.New(ip, tapo.WithPort(n)) }
	defer func() { connectKasa = old }()
	tv, e := inspectKasa("127.0.0.1")
	if e != nil || tv.Power != "PowerOn" || tv.Kind != "kasa" || tv.MAC != "11:22:33:44:55:66" {
		t.Fatalf("%+v %v", tv, e)
	}
	if e = devicePower(tv, "PowerOff"); e != nil {
		t.Fatal(e)
	}
	tv, e = inspectDevice(tv)
	if e != nil || tv.Power != "PowerOff" {
		t.Fatalf("%+v %v", tv, e)
	}
	if e = devicePower(tv, "PowerOn"); e != nil {
		t.Fatal(e)
	}
	tv, e = inspectDevice(tv)
	if e != nil || tv.Power != "PowerOn" {
		t.Fatalf("PowerOn after PowerOff: %+v %v", tv, e)
	}

}

func TestRelayErrorsRetainDeviceDetails(t *testing.T) {
	for _, c := range []struct {
		response string
		want     string
	}{
		{`{"system":{"set_relay_state":{"err_code":0}}}`, ""},
		{`{"system":{"set_relay_state":{"err_code":-1,"err_msg":"generic error"}}}`, "relay error -1: generic error"},
		{`{"error_code":-1002,"msg":"permission denied"}`, "device error -1002: permission denied"},
		{`{"system":{}}`, "device response omitted system.set_relay_state.err_code"},
	} {
		var v any
		json.Unmarshal([]byte(c.response), &v)
		e := relayCommandResult(v)
		if c.want == "" {
			if e != nil {
				t.Fatal(e)
			}
		} else if e == nil || e.Error() != c.want {
			t.Fatalf("got %v want %s", e, c.want)
		}
	}
}
