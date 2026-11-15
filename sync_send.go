package main

import (
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"time"
)

type syncPacketWriter func([]byte, *net.UDPAddr) (int, error)

func writeSyncPacket(write syncPacketWriter, packet []byte, addr *net.UDPAddr) error {
	n, e := write(packet, addr)
	if e == nil && n != len(packet) {
		return io.ErrShortWrite
	}
	return e
}

func broadcastSyncPackets(packets [][]byte, destinations map[string]bool, write syncPacketWriter) string {
	ips := make([]string, 0, len(destinations))
	for ip := range destinations {
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	sent, total := 0, 0
	var failures []string
	seenFailures := map[string]bool{}
	for _, ip := range ips {
		for _, packet := range packets {
			total++
			e := writeSyncPacket(write, packet, &net.UDPAddr{IP: net.ParseIP(ip), Port: syncPort})
			if e == nil {
				sent++
				continue
			}
			failure := fmt.Sprintf("%s: %v", ip, e)
			if !seenFailures[failure] {
				failures = append(failures, failure)
				seenFailures[failure] = true
			}
		}
	}
	// A successful UDP write means local acceptance, not confirmed delivery to
	// a peer. Continue every destination/format even after individual failures.
	if sent == 0 {
		return fmt.Sprintf("LAN sync broadcast failed (%d of %d packets sent). %s Check the network and firewall.", sent, total, strings.Join(failures, "; "))
	}
	if sent < total {
		return fmt.Sprintf("LAN sync partially sent at %s (%d of %d packets). Failed: %s", time.Now().Format("15:04:05"), sent, total, strings.Join(failures, "; "))
	}
	return fmt.Sprintf("LAN sync sent at %s (%d packets).", time.Now().Format("15:04:05"), sent)
}
