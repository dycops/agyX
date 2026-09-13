package main

// agyx — a CLI account manager in front of the Antigravity `agy` CLI.
//
// agy reads its active identity from one Windows Credential Manager entry
// (service "gemini", user "antigravity"). Only one identity fits there at a
// time, so switching accounts means swapping that entry's value. This tool
// keeps a DPAPI-encrypted vault of previously-captured credential values and
// writes the chosen one back before launching agy.
//
// It never decrypts or mints tokens: the value is moved verbatim, exactly as
// go-keyring (the same library agy uses) stored it.
//
// Commands:
//   agyx [agy args...]   pick an account, switch to it, then run agy
//                        (default agy args from config.json; --plain skips them)
//   agyx config          show / set the default agy args
//   agyx patch           region lock patch (see patch.go)
//   agyx open-config     open config.json in $EDITOR / Notepad
//   agyx ide             write the live account into Antigravity IDE (see ide.go)
//   agyx install         copy to %LOCALAPPDATA%\Programs\agyx and add it to the user PATH
//   agyx import          capture the current live credential into the vault
//   agyx list            list saved accounts
//   agyx rm <email>      remove a saved account
//   agyx current         show which saved account matches the live credential

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/zalando/go-keyring"
)

const (
	keyringService = "gemini"
	keyringUser    = "antigravity"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "import":
			label := ""
			if len(args) > 1 {
				label = args[1]
			}
			mustNil(cmdImport(label))
			return
		case "list":
			mustNil(cmdList())
			return
		case "rm":
			if len(args) < 2 {
				fatal(T("rm_usage"))
			}
			mustNil(cmdRemove(args[1]))
			return
		case "current":
			mustNil(cmdCurrent())
			return
		case "add":
			label := ""
			if len(args) > 1 {
				label = args[1]
			}
			mustNil(cmdAdd(label))
			return
		case "setup-oauth":
			mustNil(cmdSetupOAuth())
			return
		case "quota":
			mustNil(cmdQuota())
			return
		case "config":
			mustNil(cmdConfig(args[1:]))
			return
		case "patch":
			mustNil(cmdPatch())
			return
		case "open-config":
			mustNil(cmdOpenConfig())
			return
		case "ide":
			mustNil(cmdIDE())
			return
		case "install":
			mustNil(cmdInstall())
			return
		case "uninstall":
			mustNil(cmdUninstall())
			return
		case "version", "--version", "-v":
			cmdVersion()
			return
		case "-h", "--help", "help":
			printUsage()
			return
		}
	}
	// Default: interactive picker, then exec agy with the passthrough args.
	// `--plain` / `-p` as the first arg launches agy without the configured
	// default arguments.
	plain := false
	if len(args) > 0 && (args[0] == "--plain" || args[0] == "-p") {
		plain, args = true, args[1:]
	}
	mustNil(cmdSwitchAndRun(args, plain))
}

func readLive() (string, error) { return keyring.Get(keyringService, keyringUser) }
func writeLive(v string) error  { return keyring.Set(keyringService, keyringUser, v) }

func cmdImport(label string) error {
	tok, err := readLive()
	if errors.Is(err, keyring.ErrNotFound) {
		return errors.New(T("no_live"))
	}
	if err != nil {
		return err
	}
	accts, err := loadVault()
	if err != nil {
		return err
	}
	email := label
	if email == "" {
		// The credential carries no email; ask Google's userinfo endpoint.
		email = resolveEmail(tok)
	}
	if email == "" {
		// Last resort: manual label so the entry is still identifiable.
		_ = huh.NewInput().
			Title(T("label_prompt")).
			Value(&email).
			Run()
		if email == "" {
			email = fmt.Sprintf("account-%d", time.Now().Unix())
		}
	}
	accts = upsert(accts, Account{Email: email, Token: tok, AddedAt: time.Now()})
	if err := saveVault(accts); err != nil {
		return err
	}
	fmt.Printf(T("imported"), email, len(accts))
	return nil
}

func cmdList() error {
	accts, err := loadVault()
	if err != nil {
		return err
	}
	if len(accts) == 0 {
		fmt.Println(T("vault_empty"))
		return nil
	}
	live, _ := readLive()
	for _, a := range accts {
		marker := "  "
		if sameAccount(a.Token, live) {
			marker = "▶ "
		}
		fmt.Printf("%s%s\n", marker, a.Email)
	}
	return nil
}

func cmdRemove(email string) error {
	accts, err := loadVault()
	if err != nil {
		return err
	}
	out := accts[:0]
	removed := false
	for _, a := range accts {
		if a.Email == email {
			removed = true
			continue
		}
		out = append(out, a)
	}
	if !removed {
		return fmt.Errorf(T("not_found"), email)
	}
	if err := saveVault(out); err != nil {
		return err
	}
	fmt.Printf(T("removed"), email)
	return nil
}

func cmdCurrent() error {
	live, err := readLive()
	if err != nil {
		return err
	}
	accts, _ := loadVault()
	for _, a := range accts {
		if sameAccount(a.Token, live) {
			fmt.Printf(T("active_is"), a.Email)
			return nil
		}
	}
	fmt.Println(T("active_unknown"))
	return nil
}

func cmdSwitchAndRun(passthrough []string, plain bool) error {
	accts, err := loadVault()
	if err != nil {
		return err
	}
	if len(accts) == 0 {
		fmt.Println(T("vault_empty"))
		return nil
	}

	live, _ := readLive()
	choice, err := runPicker(accts, live) // quota, sorting, patch and config live in the picker
	if err != nil {
		return err
	}
	switch choice {
	case "":
		return nil // user quit
	case chooseAdd:
		return cmdAdd("")
	case chooseImport:
		return cmdImport("")
	}

	var chosen *Account
	for i := range accts {
		if accts[i].Email == choice {
			chosen = &accts[i]
			break
		}
	}
	if chosen == nil {
		return fmt.Errorf(T("not_found"), choice)
	}

	if !sameAccount(chosen.Token, live) {
		// agy refreshes the live credential as it runs; keep the vault copy of
		// the account we're leaving current, so switching back restores the
		// freshest token rather than a stale snapshot.
		for i := range accts {
			if sameAccount(accts[i].Token, live) && accts[i].Token != live {
				accts[i].Token = live
				if err := saveVault(accts); err != nil {
					return err
				}
				break
			}
		}
		if err := writeLive(chosen.Token); err != nil {
			return fmt.Errorf(T("write_store"), err)
		}
		fmt.Printf(T("switched"), chosen.Email)
	}
	if c, err := loadConfig(); err == nil && c.SwitchIDE {
		// Best effort: the IDE may be running or absent; agy still launches.
		if note, err := switchIDE(chosen.Email, chosen.Token); err != nil {
			fmt.Printf(T("ide_skipped"), err)
		} else {
			fmt.Println(note)
		}
	}
	return runAgy(passthrough, plain)
}

func agyPath() string {
	p := filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local", "agy", "bin", "agy.exe")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	if lp, err := exec.LookPath("agy"); err == nil {
		return lp
	}
	return p // let exec surface a clear error
}

// runAgy launches agy with the configured default arguments (config.json)
// followed by the passthrough args; plain skips the defaults for this launch.
func runAgy(args []string, plain bool) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}
	if !plain {
		args = append(append([]string{}, c.AgyArgs...), args...)
	}
	if c.AutoPatch {
		if note := autoPatch(); note != "" {
			fmt.Println(note)
		}
	}
	cmd := exec.Command(agyPath(), args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if !c.AgyAutoUpdate {
		cmd.Env = append(os.Environ(), "AGY_CLI_DISABLE_AUTO_UPDATE=1")
	}
	err = cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		os.Exit(exit.ExitCode())
	}
	return err
}

func printUsage() {
	fmt.Print(T("usage"))
}

func mustNil(err error) {
	if err != nil {
		fatal(err.Error())
	}
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}
