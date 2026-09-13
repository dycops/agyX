package main

// User settings: ~/.agyx/config.json — the default arguments
// prepended to every agy launch (mirroring the shell alias agy is normally
// run through) and the quota cache lifetime. Created with defaults on first
// use so it's easy to find and edit by hand; `agyx config` shows and updates
// it, `agyx open-config` opens it in an editor.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type config struct {
	AgyArgs     []string `json:"agy_args"`
	QuotaTTLSec int      `json:"quota_ttl_sec"`
	SwitchIDE   bool     `json:"switch_ide"` // also write the account into Antigravity IDE
	AutoPatch   bool     `json:"auto_patch"` // re-apply the region lock patch to agy.exe before launch
	// AgyAutoUpdate lets agy update itself; false launches agy with
	// AGY_CLI_DISABLE_AUTO_UPDATE=1 so a patched binary stays patched.
	AgyAutoUpdate bool `json:"agy_autoupdate"`
}

const defaultQuotaTTLSec = 60

func defaultConfig() config {
	return config{
		AgyArgs:     []string{"--dangerously-skip-permissions"},
		QuotaTTLSec: defaultQuotaTTLSec,
		SwitchIDE:   true,
		AutoPatch:   false,
		// Off by default: agy's self-update replaces the patched binary.
		AgyAutoUpdate: false,
	}
}

func configPath() string {
	return filepath.Join(vaultDir(), "config.json")
}

// loadConfig reads the config, writing the defaults first if it doesn't exist.
func loadConfig() (config, error) {
	data, err := os.ReadFile(configPath())
	if errors.Is(err, os.ErrNotExist) {
		c := defaultConfig()
		return c, saveConfig(c)
	}
	if err != nil {
		return config{}, err
	}
	c := defaultConfig()
	if err := json.Unmarshal(data, &c); err != nil {
		return config{}, fmt.Errorf("%s: %w", configPath(), err)
	}
	if c.AgyArgs == nil {
		c.AgyArgs = []string{}
	}
	if c.QuotaTTLSec < 0 {
		c.QuotaTTLSec = 0
	}
	// Rewrite the file if a newer field is missing from it, so every setting
	// is visible for editing.
	var onDisk map[string]json.RawMessage
	if json.Unmarshal(data, &onDisk) == nil {
		for _, key := range []string{"agy_args", "quota_ttl_sec", "switch_ide", "auto_patch", "agy_autoupdate"} {
			if _, ok := onDisk[key]; !ok {
				return c, saveConfig(c)
			}
		}
	}
	return c, nil
}

// quotaTTL is the cache lifetime from config (0 = always refetch).
func quotaTTL() time.Duration {
	c, err := loadConfig()
	if err != nil {
		return defaultQuotaTTLSec * time.Second
	}
	return time.Duration(c.QuotaTTLSec) * time.Second
}

func saveConfig(c config) error {
	if err := os.MkdirAll(vaultDir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), append(data, '\n'), 0o600)
}

// cmdConfig: `agyx config` shows the settings; `agyx config args [a b ...]`
// replaces the default agy arguments (no values = launch agy plain).
func cmdConfig(args []string) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}
	if len(args) > 0 {
		if args[0] != "args" {
			return errors.New(T("cfg_usage"))
		}
		c.AgyArgs = args[1:]
		if err := saveConfig(c); err != nil {
			return err
		}
	}
	fmt.Printf("%s\n", configPath())
	fmt.Printf("  agy_args:      %s\n", quoteArgs(c.AgyArgs))
	fmt.Printf("  quota_ttl_sec: %d\n", c.QuotaTTLSec)
	fmt.Printf("  switch_ide:    %v\n", c.SwitchIDE)
	fmt.Printf("  auto_patch:    %v\n", c.AutoPatch)
	fmt.Printf("  agy_autoupdate: %v\n", c.AgyAutoUpdate)
	return nil
}

// editorCmd returns the command that opens config.json in $EDITOR (or
// Notepad); the file is created with defaults first if missing.
func editorCmd() (*exec.Cmd, error) {
	if _, err := loadConfig(); err != nil {
		return nil, err
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "notepad"
	}
	return exec.Command(editor, configPath()), nil
}

// cmdOpenConfig opens config.json in the editor and waits for it.
func cmdOpenConfig() error {
	cmd, err := editorCmd()
	if err != nil {
		return err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func quoteArgs(a []string) string {
	if len(a) == 0 {
		return T("cfg_none")
	}
	return strings.Join(a, " ")
}
