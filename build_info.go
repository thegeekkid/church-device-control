package main

import (
	"fmt"
	"runtime/debug"
)

// Official releases set this using -X and record standard Go VCS metadata.
// Archive/local builds may lack .git and are clearly identified as such.
var appVersion = "development"

func applicationBuildDetails() string {
	info, _ := debug.ReadBuildInfo()
	return formatBuildDetails(info)
}

func formatBuildDetails(info *debug.BuildInfo) string {
	revision, modified := "not recorded (development/source archive build)", ""
	if info != nil {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
			case "vcs.modified":
				if setting.Value == "true" {
					modified = "\r\nSource changes: modified"
				}
			}
		}
	}
	return fmt.Sprintf("Version: %s\r\nSource commit: %s%s", appVersion, revision, modified)
}
