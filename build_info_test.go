package main

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestBuildDetailsRecordsCommitAndSourceChanges(t *testing.T) {
	const commit = "0123456789abcdef0123456789abcdef01234567"
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: commit}, {Key: "vcs.modified", Value: "true"}}}
	text := formatBuildDetails(info)
	if !strings.Contains(text, "Version: "+appVersion) || !strings.Contains(text, "Source commit: "+commit) || !strings.Contains(text, "Source changes: modified") {
		t.Fatal(text)
	}
	info.Settings[1].Value = "false"
	if text := formatBuildDetails(info); strings.Contains(text, "Source changes: modified") {
		t.Fatal("clean build reported modified", text)
	}
	if text := formatBuildDetails(nil); !strings.Contains(text, "not recorded") {
		t.Fatal("source archive build invented provenance", text)
	}
}
