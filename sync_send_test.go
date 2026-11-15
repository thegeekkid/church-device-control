package main

import (
	"errors"
	"net"
	"strings"
	"testing"
)

func TestSyncBroadcastReportsPartialFailureAndAttemptsEveryFormat(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail func(string, string) (bool, bool)
		want []string
	}{
		{"success", func(string, string) (bool, bool) { return false, false }, []string{"LAN sync sent at", "4 packets"}},
		{"partial", func(ip, _ string) (bool, bool) { return ip == "198.51.100.255", false }, []string{"LAN sync partially sent", "2 of 4 packets", "198.51.100.255: blocked"}},
		{"failed", func(string, string) (bool, bool) { return true, false }, []string{"LAN sync broadcast failed", "0 of 4 packets", "192.0.2.255: blocked", "198.51.100.255: blocked"}},
		{"short write", func(_, packet string) (bool, bool) { return false, packet == "v1" }, []string{"LAN sync partially sent", "2 of 4 packets", "short write"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := map[string]int{}
			write := func(packet []byte, addr *net.UDPAddr) (int, error) {
				if addr.Port != syncPort {
					t.Fatal("incorrect sync port", addr)
				}
				calls[addr.IP.String()+"/"+string(packet)]++
				fail, short := tc.fail(addr.IP.String(), string(packet))
				if fail {
					return 0, errors.New("blocked")
				}
				if short {
					return len(packet) - 1, nil
				}
				return len(packet), nil
			}
			status := broadcastSyncPackets([][]byte{[]byte("v2"), []byte("v1")}, map[string]bool{"192.0.2.255": true, "198.51.100.255": true}, write)
			for _, want := range tc.want {
				if !strings.Contains(status, want) {
					t.Errorf("%q does not contain %q", status, want)
				}
			}
			for _, ip := range []string{"192.0.2.255", "198.51.100.255"} {
				for _, format := range []string{"v2", "v1"} {
					if calls[ip+"/"+format] != 1 {
						t.Fatal("skipped or duplicated a destination/format", calls)
					}
				}
			}
		})
	}
}
