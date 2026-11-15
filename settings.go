package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var secrets struct {
	sync.Mutex
	// Windows DPAPI protects the file (secrets_windows.go). The local Kasa
	// client and sync cipher need decrypted values while this process runs;
	// neither passwords nor this key are placed in sharedConfig or LAN JSON.
	User, Password, SyncKey string
}

type localSecrets struct{ User, Password, SyncKey string }

func secretPath() string { return filepath.Join(filepath.Dir(configPath()), "local-secrets.bin") }
func saveSecrets(u, p, k string) error {
	plain, e := json.Marshal(localSecrets{u, p, k})
	if e != nil {
		return e
	}
	defer clear(plain)
	b, e := protect(plain)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(secretPath()), 0700); e != nil {
		return e
	}
	if e = os.WriteFile(secretPath(), b, 0600); e != nil {
		return e
	}
	secrets.Lock()
	changed := secrets.User != u || secrets.Password != p
	secrets.User = u
	secrets.Password = p
	secrets.SyncKey = k
	secrets.Unlock()
	if changed {
		// Drop references to clients that retain the old account credentials.
		kasaSessions.Lock()
		clear(kasaSessions.ByIP)
		kasaSessions.Unlock()
	}
	return nil
}
func loadSecrets() error {
	b, e := os.ReadFile(secretPath())
	if os.IsNotExist(e) {
		return nil // Normal first run: no credentials have been saved yet.
	}
	if e != nil {
		return fmt.Errorf("read saved credentials: %w", e)
	}
	plain, e := unprotect(b)
	if e != nil {
		return fmt.Errorf("decrypt saved credentials for this Windows user: %w", e)
	}
	defer clear(plain)
	var s *localSecrets
	if json.Unmarshal(plain, &s) != nil || s == nil {
		// Do not include plaintext or parser snippets in an error dialog/log.
		return fmt.Errorf("saved credentials contain invalid data")
	}
	secrets.Lock()
	secrets.User = s.User
	secrets.Password = s.Password
	secrets.SyncKey = s.SyncKey
	secrets.Unlock()
	return nil
}
