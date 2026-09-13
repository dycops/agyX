package main

// Tiny i18n: pick English or Russian from AGYX_LANG / LC_ALL / LC_MESSAGES /
// LANG (default English), and look messages up by key. T() returns the string
// or format for the active language.

import (
	"os"
	"strings"
)

type lang int

const (
	en lang = 0
	ru lang = 1
)

var curLang = detectLang()

func detectLang() lang {
	for _, k := range []string{"AGYX_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := strings.ToLower(os.Getenv(k)); v != "" {
			if strings.HasPrefix(v, "ru") {
				return ru
			}
			return en
		}
	}
	return en
}

func T(key string) string {
	if m, ok := messages[key]; ok {
		return m[curLang]
	}
	return key
}

// messages maps a key to {english, russian}.
var messages = map[string][2]string{
	"tagline":    {"cli account manager", "cli аккаунт-менеджер"},
	"k_select":   {"select", "выбрать"},
	"k_run":      {"run", "запустить"},
	"k_quit":     {"quit", "выход"},
	"k_refresh":  {"refresh quota", "обновить квоту"},
	"k_delete":   {"delete", "удалить"},
	"del_ask":    {"delete account?", "удалить аккаунт?"},
	"del_done":   {"✓ removed %s", "✓ удалён %s"},
	"del_err":    {"! delete failed: %v", "! не удалось удалить: %v"},
	"act_add":    {"add account", "добавить аккаунт"},
	"act_impt":   {"import current", "импортировать текущий"},
	"act_patch":  {"region lock patch", "патч region lock"},
	"act_cfg":    {"open config", "открыть конфиг"},
	"h_account":  {"account", "аккаунт"},
	"reset_soon": {"soon", "скоро"},
	"u_d":        {"d", "д"},
	"u_h":        {"h", "ч"},
	"u_m":        {"m", "м"},

	"vault_empty":    {"vault is empty. Sign in to Antigravity, then run: agyx import", "vault пуст. Войди в Antigravity и выполни: agyx import"},
	"no_live":        {"no active credential — sign in first (agyx add, or log in to Antigravity), then: agyx import", "нет активного креда — сначала войди (agyx add или вход в Antigravity), потом: agyx import"},
	"label_prompt":   {"email not detected — set a label (e.g. work / personal)", "email не определился — задай метку (напр. work / personal)"},
	"imported":       {"✓ imported: %s  (in vault: %d)\n", "✓ импортирован: %s  (в vault: %d)\n"},
	"not_found":      {"not found: %s", "не найден: %s"},
	"removed":        {"✓ removed: %s\n", "✓ удалён: %s\n"},
	"active_is":      {"active: %s\n", "активен: %s\n"},
	"active_unknown": {"active account is not in the vault (import it: agyx import)", "активный аккаунт не в vault (импортируй: agyx import)"},
	"write_store":    {"failed to write credential store: %w", "не удалось записать в credential store: %w"},
	"switched":       {"✓ switched to %s\n", "✓ переключено на %s\n"},
	"rm_usage":       {"usage: agyx rm <email>", "использование: agyx rm <email>"},
	"cfg_usage":      {"usage: agyx config [args <agy args...>]", "использование: agyx config [args <аргументы agy...>]"},
	"cfg_none":       {"(none)", "(нет)"},

	"patch_stop":     {"stopping Antigravity / agy / language_server…", "останавливаю Antigravity / agy / language_server…"},
	"patch_env_ok":   {"✓ CLOUD_CODE_URL set, proxy variables cleared (user environment)", "✓ CLOUD_CODE_URL задан, прокси-переменные очищены (окружение пользователя)"},
	"patch_env_fail": {"! environment not updated: %v", "! окружение не обновлено: %v"},
	"patch_none":     {"no binaries found to patch", "бинарники для патча не найдены"},
	"patch_done":     {"✓ patched  %s (%d)", "✓ пропатчен  %s (%d)"},
	"patch_skip":     {"· ok  %s", "· ок  %s"},
	"patch_err":      {"! error  %s: %v", "! ошибка  %s: %v"},
	"patch_busy":     {"patching…", "патчу…"},
	"patch_auto":     {"✓ agy.exe was updated — region lock patch re-applied (%d)", "✓ agy.exe обновился — патч region lock применён заново (%d)"},
	"cfg_busy":       {"editing config…", "редактирую конфиг…"},
	"cfg_saved":      {"✓ config reloaded", "✓ конфиг перечитан"},
	"cfg_err":        {"! editor: %v", "! редактор: %v"},
	"loading":        {"loading quota…", "загружаю квоту…"},

	"inst_copied":       {"✓ installed: %s\n", "✓ установлен: %s\n"},
	"inst_busy":         {"is agyx running from the install folder? close it and retry", "agyx запущен из папки установки? закрой и повтори"},
	"inst_path_added":   {"✓ added to user PATH: %s\n  open a new terminal to use `agyx`\n", "✓ добавлен в PATH пользователя: %s\n  открой новый терминал, чтобы работало `agyx`\n"},
	"inst_path_present": {"· already on PATH", "· уже в PATH"},
	"uninst_path":       {"✓ removed from user PATH", "✓ убран из PATH пользователя"},
	"uninst_done":       {"✓ removed %s\n  (~/.agyx with the vault and config is kept)\n", "✓ удалён %s\n  (~/.agyx с vault и конфигом оставлен)\n"},

	"ide_running":  {"Antigravity IDE is running — close it first", "Antigravity IDE запущена — сначала закрой её"},
	"ide_switched": {"✓ Antigravity IDE → %s (takes effect on next IDE start)", "✓ Antigravity IDE → %s (применится при следующем запуске IDE)"},
	"ide_skipped":  {"! IDE not switched: %v\n", "! IDE не переключена: %v\n"},

	"add_opening":    {"Opening the browser for Google sign-in…", "Открываю браузер для входа в Google…"},
	"add_manual":     {"If it didn't open, go to this link manually:", "Если не открылось — перейди по ссылке вручную:"},
	"add_done_html":  {"<h2>Done ✓</h2><p>Account added. You can close this tab and return to the terminal.</p>", "<h2>Готово ✓</h2><p>Аккаунт добавлен. Можно закрыть вкладку и вернуться в терминал.</p>"},
	"add_login_err":  {"Sign-in error: %s. You can close the tab.", "Ошибка входа: %s. Можно закрыть вкладку."},
	"no_secret":      {"no client_secret — run first: agyx setup-oauth (or fill ~/.agyx/oauth.json)", "нет client_secret — выполни сначала: agyx setup-oauth (или заполни ~/.agyx/oauth.json)"},
	"state_mismatch": {"state mismatch (possible CSRF)", "несовпадение state (возможна CSRF-попытка)"},
	"add_timeout":    {"sign-in wait timed out (5 min)", "тайм-аут ожидания входа (5 мин)"},
	"exchange_fail":  {"code exchange failed: %w", "обмен кода на токен не удался: %w"},
	"added_active":   {"✓ added and activated: %s\n", "✓ добавлен и активирован: %s\n"},

	"setup_written":   {"✓ oauth.json written: %s\n", "✓ oauth.json записан: %s\n"},
	"setup_source":    {"  source: %s\n", "  источник: %s\n"},
	"setup_id":        {"  client_id: %s\n", "  client_id: %s\n"},
	"setup_secret":    {"  client_secret: captured (%d chars)\n", "  client_secret: захвачен (%d символов)\n"},
	"setup_method":    {"  auth_method: %q\n", "  auth_method: %q\n"},
	"no_secret_found": {"client_secret not found (searched agy.exe / Antigravity IDE / Manager)", "client_secret не найден (искал в agy.exe / Antigravity IDE / Manager)"},

	"usage": {
		`agyx — CLI account manager for agy

  agyx                 pick an account and launch agy with the default args
  agyx <agy args...>   same, extra args appended for agy
  agyx --plain [...]   launch agy without the default args (-p)
  agyx config          show the default agy args (~/.agyx/config.json)
  agyx config args ... set the default agy args (no values = none)
  agyx patch           region lock patch: stop Antigravity, set CLOUD_CODE_URL, patch binaries
  agyx open-config     open ~/.agyx/config.json in your editor
  agyx ide             write the active account into Antigravity IDE (IDE must be closed;
                       "switch_ide": true in config does this on every switch)
  agyx add [label]     sign a new account in via the browser and save it
  agyx setup-oauth     one-time: extract the OAuth client into oauth.json
  agyx import [label]  save the currently signed-in account (label / auto-email)
  agyx list            list saved accounts
  agyx rm <email>      remove an account from the vault
  agyx current         show the active account
  agyx install         copy agyx to %LOCALAPPDATA%\Programs\agyx and add it to PATH
  agyx uninstall       remove that copy and the PATH entry (vault stays)
  agyx version         print version
`,
		`agyx — CLI аккаунт-менеджер для agy

  agyx                 выбрать аккаунт и запустить agy с аргументами по умолчанию
  agyx <agy args...>   то же, доп. аргументы добавляются для agy
  agyx --plain [...]   запустить agy без аргументов по умолчанию (-p)
  agyx config          показать аргументы по умолчанию (~/.agyx/config.json)
  agyx config args ... задать аргументы по умолчанию (без значений = никаких)
  agyx patch           патч region lock: остановить Antigravity, задать CLOUD_CODE_URL, пропатчить бинарники
  agyx open-config     открыть ~/.agyx/config.json в редакторе
  agyx ide             записать активный аккаунт в Antigravity IDE (IDE должна быть закрыта;
                       "switch_ide": true в конфиге делает это при каждом свитче)
  agyx add [метка]     войти в новый аккаунт через браузер и сохранить
  agyx setup-oauth     разово извлечь OAuth-клиент в oauth.json
  agyx import [метка]  сохранить текущий залогиненный аккаунт (метка / авто-email)
  agyx list            список сохранённых аккаунтов
  agyx rm <email>      удалить аккаунт из vault
  agyx current         показать активный аккаунт
  agyx install         скопировать agyx в %LOCALAPPDATA%\Programs\agyx и добавить в PATH
  agyx uninstall       убрать копию и запись в PATH (vault остаётся)
  agyx version         показать версию
`,
	},
}
