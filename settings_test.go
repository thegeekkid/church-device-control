package main

import (
	"os"
	"testing"
)

func TestCredentialLoadDistinguishesFirstRunFromReadFailure(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if e := loadSecrets(); e != nil {
		t.Fatal("first run should not warn", e)
	}
	if e := os.MkdirAll(secretPath(), 0700); e != nil {
		t.Fatal(e)
	}
	if e := loadSecrets(); e == nil {
		t.Fatal("credential read failure was silently ignored")
	}
}
