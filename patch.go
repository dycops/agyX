package main

// `agyx patch` — the region lock patch, ported from fix-and-run-agy.ps1:
//  1. stop running Antigravity processes (they hold the binaries open),
//  2. point the client at the daily Cloud Code endpoint via the user
//     environment and clear stale proxy variables,
//  3. byte-patch the known binaries so the region-eligibility string never
//     matches ("ineligible" → "inexigible", same length, .bak kept once).
// Idempotent: already-patched files are reported and left alone.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const cloudCodeURL = "https://daily-cloudcode-pa.googleapis.com"

var (
	patchFrom = []byte("ineligible")
	patchTo   = []byte("inexigible")

	patchProcs = []string{"antigravity.exe", "agy.exe", "language_server.exe"}
)

// patchTargets lists every binary that may carry the eligibility check.
func patchTargets() []string {
	home, la := os.Getenv("USERPROFILE"), os.Getenv("LOCALAPPDATA")
	cands := []string{
		agyPath(),
		filepath.Join(home, ".local", "bin", "agy.exe"),
		filepath.Join(home, ".gemini", "antigravity-cli", "bin", "antigravity.exe"),
		filepath.Join(home, ".gemini", "antigravity-cli", "bin", "agy.exe"),
		filepath.Join(la, "Programs", "Antigravity", "resources", "bin", "language_server.exe"),
		filepath.Join(la, "Programs", "Antigravity", "resources", "bin", "language_server_windows_x64.exe"),
		filepath.Join(la, "Programs", "Antigravity", "Antigravity.exe"),
	}
	for _, name := range []string{"agy", "antigravity", "language_server"} {
		if p, err := exec.LookPath(name); err == nil {
			cands = append(cands, p)
		}
	}
	seen := map[string]bool{}
	var out []string
	for _, c := range cands {
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		key := filepath.Clean(abs)
		if seen[key] {
			continue
		}
		if st, err := os.Stat(key); err != nil || st.IsDir() {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

func stopProcesses() {
	for _, p := range patchProcs {
		// taskkill exits non-zero when nothing matched; that's fine.
		_ = exec.Command("taskkill", "/IM", p, "/F", "/T").Run()
	}
}

// setUserEnv writes the persistent user environment (HKCU\Environment) and
// mirrors it into this process so the agy we launch inherits it.
func setUserEnv() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.SetStringValue("CLOUD_CODE_URL", cloudCodeURL); err != nil {
		return err
	}
	_ = os.Setenv("CLOUD_CODE_URL", cloudCodeURL)
	for _, v := range []string{"HTTPS_PROXY", "HTTP_PROXY", "ALL_PROXY"} {
		if err := k.DeleteValue(v); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		_ = os.Unsetenv(v)
	}
	return nil
}

// patchFile rewrites every occurrence in place; returns the count (0 = already
// patched or nothing to do). An unpatched file is pristine (fresh install or
// self-update), so it is saved as .bak before being rewritten.
func patchFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	n := bytes.Count(data, patchFrom)
	if n == 0 {
		return 0, nil
	}
	if err := os.WriteFile(path+".bak", data, 0o644); err != nil {
		return 0, err
	}
	patched := bytes.ReplaceAll(data, patchFrom, patchTo)
	return n, os.WriteFile(path, patched, 0o755)
}

// autoPatch re-applies the binary patch to agy.exe right before launch: agy
// updates itself in place and the new build arrives unpatched. Returns a
// note when something happened, "" when the binary was already fine.
func autoPatch() string {
	n, err := patchFile(agyPath())
	switch {
	case err != nil:
		return fmt.Sprintf(T("patch_err"), agyPath(), err)
	case n > 0:
		return fmt.Sprintf(T("patch_auto"), n)
	}
	return ""
}

// runPatch performs the patch, reporting each step through emit (one line
// each, no trailing newline) so the CLI and the TUI can both show it.
func runPatch(emit func(string)) {
	emit(T("patch_stop"))
	stopProcesses()

	if err := setUserEnv(); err != nil {
		emit(fmt.Sprintf(T("patch_env_fail"), err))
	} else {
		emit(T("patch_env_ok"))
	}

	targets := patchTargets()
	if len(targets) == 0 {
		emit(T("patch_none"))
		return
	}
	for _, t := range targets {
		n, err := patchFile(t)
		switch {
		case err != nil:
			emit(fmt.Sprintf(T("patch_err"), t, err))
		case n > 0:
			emit(fmt.Sprintf(T("patch_done"), t, n))
		default:
			emit(fmt.Sprintf(T("patch_skip"), t))
		}
	}
}

func cmdPatch() error {
	runPatch(func(s string) { fmt.Println(s) })
	return nil
}
