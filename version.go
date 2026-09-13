package main

// Build metadata, stamped by the build script via -ldflags:
//   go build -ldflags "-X main.version=1.0.0 -X main.commit=abc1234"
// A plain `go build` yields "dev".

import (
	"fmt"
	"runtime"
)

var (
	version = "dev"
	commit  = ""
)

func versionString() string {
	if commit != "" {
		return version + " (" + commit + ")"
	}
	return version
}

func cmdVersion() {
	fmt.Printf("agyx %s  %s/%s  %s\n", versionString(), runtime.GOOS, runtime.GOARCH, runtime.Version())
}
