//go:build !windows

package main

import "fmt"

func protect([]byte) ([]byte, error) {
	return nil, fmt.Errorf("credential storage requires Windows DPAPI")
}
func unprotect([]byte) ([]byte, error) {
	return nil, fmt.Errorf("credential storage requires Windows DPAPI")
}
