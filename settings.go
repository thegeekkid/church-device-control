package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

var secrets struct {
	sync.Mutex
	User, Password, SyncKey string
}

type localSecrets struct{ User, Password, SyncKey string }

func secretPath() string { return filepath.Join(filepath.Dir(configPath()), "local-secrets.bin") }
func saveSecrets(u, p, k string) error {
	b, e := json.Marshal(localSecrets{u, p, k})
	if e != nil {
		return e
	}
	b, e = protect(b)
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
	secrets.User = u
	secrets.Password = p
	secrets.SyncKey = k
	secrets.Unlock()
	return nil
}
func loadSecrets() {
	b, e := os.ReadFile(secretPath())
	if e != nil {
		return
	}
	b, e = unprotect(b)
	if e != nil {
		return
	}
	var s localSecrets
	if json.Unmarshal(b, &s) != nil {
		return
	}
	secrets.Lock()
	secrets.User = s.User
	secrets.Password = s.Password
	secrets.SyncKey = s.SyncKey
	secrets.Unlock()
}
