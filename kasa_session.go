package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/pborges/tapo"
	"sync"
)

type kasaSession struct {
	Client      *tapo.Strip
	Credentials [32]byte
}

var kasaSessions = struct {
	sync.Mutex
	ByIP map[string]kasaSession
}{ByIP: map[string]kasaSession{}}

func credentialFingerprint() [32]byte {
	secrets.Lock()
	defer secrets.Unlock()
	return sha256.Sum256([]byte(secrets.User + "\x00" + secrets.Password))
}
func rememberKasa(ip string, c *tapo.Strip, identity [32]byte) {
	kasaSessions.Lock()
	defer kasaSessions.Unlock()
	kasaSessions.ByIP[ip] = kasaSession{c, identity}
}
func probeKasa(ctx context.Context, ip string) (*tapo.Strip, tapo.RawResponse, error) {
	identity := credentialFingerprint()
	kasaSessions.Lock()
	cached, ok := kasaSessions.ByIP[ip]
	kasaSessions.Unlock()
	if ok && cached.Credentials == identity {
		r, e := cached.Client.Query(ctx, map[string]any{"system": map[string]any{"get_sysinfo": nil}})
		if e == nil {
			return cached.Client, r, nil
		}
		var auth *tapo.AuthenticationError
		if !errors.As(e, &auth) {
			return nil, nil, fmt.Errorf("Kasa session read failed: %w", e)
		}
		// Renegotiate only an authentication mismatch. The client itself renews
		// expired/invalidated sessions on HTTP 403; transient I/O errors are retried
		// by the status reader without discarding an authenticated connection.
		kasaSessions.Lock()
		delete(kasaSessions.ByIP, ip)
		kasaSessions.Unlock()
	}
	c, r, e := negotiateKasa(ctx, ip)
	if e == nil {
		rememberKasa(ip, c, identity)
	}
	return c, r, e
}
