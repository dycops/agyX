package main

// `agyx install` — copy the running binary to the per-user programs folder
// and put that folder on the user PATH (no admin needed):
//   %LOCALAPPDATA%\Programs\agyx\agyx.exe
// `agyx uninstall` reverses both. PATH lives in HKCU\Environment; after
// writing it we broadcast WM_SETTINGCHANGE so new shells see it.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

func installDir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "agyx")
}

func cmdInstall() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	dst := filepath.Join(installDir(), "agyx.exe")
	if s, _ := filepath.Abs(self); !strings.EqualFold(s, dst) {
		if err := os.MkdirAll(installDir(), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(self)
		if err != nil {
			return err
		}
		// Write next to the target and rename, so a running copy is replaced
		// atomically (Windows allows renaming over a file that is not open).
		tmp := dst + ".new"
		if err := os.WriteFile(tmp, data, 0o755); err != nil {
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			return fmt.Errorf("%w (%s)", err, T("inst_busy"))
		}
	}
	fmt.Printf(T("inst_copied"), dst)

	added, err := userPathAdd(installDir())
	if err != nil {
		return err
	}
	if added {
		fmt.Printf(T("inst_path_added"), installDir())
	} else {
		fmt.Println(T("inst_path_present"))
	}
	return nil
}

func cmdUninstall() error {
	removed, err := userPathRemove(installDir())
	if err != nil {
		return err
	}
	if removed {
		fmt.Println(T("uninst_path"))
	}
	if err := os.RemoveAll(installDir()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fmt.Printf(T("uninst_done"), installDir())
	return nil
}

// --- user PATH -------------------------------------------------------------

func readUserPath(k registry.Key) (string, bool) {
	v, typ, err := k.GetStringValue("Path")
	if err != nil {
		return "", false
	}
	return v, typ == registry.EXPAND_SZ
}

func writeUserPath(k registry.Key, v string, expand bool) error {
	if expand {
		return k.SetExpandStringValue("Path", v)
	}
	return k.SetStringValue("Path", v)
}

func splitPath(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ";") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func pathHas(parts []string, dir string) bool {
	for _, p := range parts {
		if strings.EqualFold(filepath.Clean(os.ExpandEnv(p)), filepath.Clean(dir)) {
			return true
		}
	}
	return false
}

func userPathAdd(dir string) (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	cur, expand := readUserPath(k)
	parts := splitPath(cur)
	if pathHas(parts, dir) {
		return false, nil
	}
	parts = append(parts, dir)
	if err := writeUserPath(k, strings.Join(parts, ";"), expand); err != nil {
		return false, err
	}
	broadcastEnvChange()
	return true, nil
}

func userPathRemove(dir string) (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	cur, expand := readUserPath(k)
	var kept []string
	removed := false
	for _, p := range splitPath(cur) {
		if strings.EqualFold(filepath.Clean(os.ExpandEnv(p)), filepath.Clean(dir)) {
			removed = true
			continue
		}
		kept = append(kept, p)
	}
	if !removed {
		return false, nil
	}
	if err := writeUserPath(k, strings.Join(kept, ";"), expand); err != nil {
		return false, err
	}
	broadcastEnvChange()
	return true, nil
}

// broadcastEnvChange tells running Explorer/shells the environment changed
// (WM_SETTINGCHANGE with "Environment"); best effort.
func broadcastEnvChange() {
	const (
		hwndBroadcast   = 0xffff
		wmSettingChange = 0x001A
		smtoAbortIfHung = 0x0002
	)
	env, _ := syscall.UTF16PtrFromString("Environment")
	user32 := syscall.NewLazyDLL("user32.dll")
	send := user32.NewProc("SendMessageTimeoutW")
	var res uintptr
	_, _, _ = send.Call(hwndBroadcast, wmSettingChange, 0,
		uintptr(unsafe.Pointer(env)), smtoAbortIfHung, 5000, uintptr(unsafe.Pointer(&res)))
}
