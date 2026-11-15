package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/pborges/tapo"
	"sync"
	"time"
)

type kasaSession struct {
	Client       *tapo.Strip
	Credentials  [32]byte
	AuthFailures int
	RetryAt      time.Time
	AuthError    error
}

var kasaSessions = struct {
	sync.Mutex
	ByIP map[string]kasaSession
}{ByIP: map[string]kasaSession{}}

func credentialFingerprint() [32]byte {
	// Local cache invalidation only: this digest is neither stored on disk nor
	// transmitted. KLAP authentication is performed by the tapo client, not by
	// comparing this fingerprint with anything supplied by a device.
	secrets.Lock()
	defer secrets.Unlock()
	return sha256.Sum256([]byte(secrets.User + "\x00" + secrets.Password))
}
func rememberKasa(ip string, c *tapo.Strip, identity [32]byte) {
	kasaSessions.Lock()
	defer kasaSessions.Unlock()
	kasaSessions.ByIP[ip] = kasaSession{Client: c, Credentials: identity}
}

type kasaAuthCooldownError struct {
	RetryAt time.Time
	Cause   error
}

func (e *kasaAuthCooldownError) Error() string {
	wait := time.Until(e.RetryAt).Round(time.Second)
	if wait < 0 {
		wait = 0
	}
	return fmt.Sprintf("Kasa authentication paused for %s. Change credentials or Discover devices to retry immediately. Last error: %v", wait, e.Cause)
}
func (e *kasaAuthCooldownError) Unwrap() error { return e.Cause }

func isKasaAuthenticationError(e error) bool {
	var auth *tapo.AuthenticationError
	return errors.As(e, &auth)
}

func rememberKasaAuthFailure(ip string, identity [32]byte, cause error) error {
	kasaSessions.Lock()
	defer kasaSessions.Unlock()
	old := kasaSessions.ByIP[ip]
	failures := 1
	if old.Credentials == identity {
		failures += old.AuthFailures
	}
	if failures > 5 {
		failures = 5
	}
	wait := 30 * time.Second * time.Duration(1<<uint(failures-1))
	if wait > 5*time.Minute {
		wait = 5 * time.Minute
	}
	retryAt := time.Now().Add(wait)
	kasaSessions.ByIP[ip] = kasaSession{Credentials: identity, AuthFailures: failures, RetryAt: retryAt, AuthError: cause}
	return &kasaAuthCooldownError{RetryAt: retryAt, Cause: cause}
}

func retryFailedKasaAuthentication() {
	// Only explicit discovery resets failed attempts. Periodic refreshes and
	// power commands honor the cooldown; a changed credential fingerprint also
	// bypasses it, and saveSecrets clears cached entries after account changes.
	kasaSessions.Lock()
	defer kasaSessions.Unlock()
	for ip, entry := range kasaSessions.ByIP {
		if entry.AuthFailures > 0 {
			delete(kasaSessions.ByIP, ip)
		}
	}
}

func probeKasa(ctx context.Context, ip string) (*tapo.Strip, tapo.RawResponse, error) {
	if e := ctx.Err(); e != nil {
		return nil, nil, e
	}
	identity := credentialFingerprint()
	kasaSessions.Lock()
	cached, ok := kasaSessions.ByIP[ip]
	kasaSessions.Unlock()
	if ok && cached.Credentials == identity {
		// Only authentication failures create this delay, never offline/I/O
		// failures. Return immediately rather than sleeping in a refresh worker.
		if time.Now().Before(cached.RetryAt) {
			return nil, nil, &kasaAuthCooldownError{RetryAt: cached.RetryAt, Cause: cached.AuthError}
		}
	}
	if ok && cached.Credentials == identity && cached.Client != nil {
		// IP selects a cached client; it is not proof of peer identity. The tapo
		// KLAP transport validates handshake/response signatures and renews HTTP
		// 403 sessions. Legacy XOR devices have no peer authentication protocol.
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
	} else if isKasaAuthenticationError(e) {
		e = rememberKasaAuthFailure(ip, identity, e)
	}
	return c, r, e
}
