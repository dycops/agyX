# Changelog

All notable changes to agyX. Versions follow [SemVer](https://semver.org).

## [1.0.0] — 2026-09-13

First tagged release.

### Picker
- Alternate-screen TUI: one row per account with Gemini and Claude/GPT quota
  (5h window and weekly), reset countdowns, thin bars, async loading spinner.
- Accounts sorted by Gemini 5h remaining (least first).
- `r` refreshes quota past the cache, `d` deletes an account with an inline
  `y/n` confirmation.
- Actions: add account (browser OAuth), import current, region lock patch,
  open config — the last two run inside the picker with a log panel.

### Switching
- Swaps the `gemini:antigravity` Credential Manager slot that `agy` reads.
- Also writes the account into Antigravity IDE's `state.vscdb`
  (`switch_ide`, on by default; IDE must be closed).
- Keeps the vault copy of the account being switched away from fresh.

### Configuration (`~/.agyx/config.json`)
- `agy_args` — default arguments for `agy` (`--dangerously-skip-permissions`).
- `quota_ttl_sec` — quota cache lifetime (60).
- `switch_ide`, `auto_patch`, `agy_autoupdate` — see README.

### Install
- `agyx install` / `uninstall`: per-user install to `%LOCALAPPDATA%\Programs\agyx`
  with a user PATH entry; `build.ps1` stamps the version from git.

### Region lock
- `agyx patch`: stops Antigravity processes, sets `CLOUD_CODE_URL`, clears
  proxy variables, byte-patches the eligibility check in the binaries.
- `agy_autoupdate: false` launches `agy` with `AGY_CLI_DISABLE_AUTO_UPDATE=1`
  so a patched binary is not replaced; `auto_patch` re-applies after updates.

### Security
- OAuth loopback receiver verifies `state` before anything else, escapes the
  reflected error, listens and redirects on `127.0.0.1`.
- Vault is DPAPI-encrypted; tokens are never printed.
- English / Russian interface (`AGYX_LANG`).
