<p align="center">
  <img src="assets/logo.svg" width="96" alt="agyX">
</p>

<h1 align="center">agyX</h1>

<p align="center">
  CLI аккаунт-менеджер для Antigravity <code>agy</code> и IDE под Windows.<br>
  Выбрал аккаунт, увидел квоту, запустил — две клавиши.
</p>

<p align="center">
  <a href="CHANGELOG.md"><img alt="version" src="https://img.shields.io/badge/version-1.0.0-2f80ed"></a>
  <img alt="go" src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white">
  <img alt="platform" src="https://img.shields.io/badge/platform-Windows-0078D6?logo=windows&logoColor=white">
  <img alt="ui" src="https://img.shields.io/badge/TUI-Bubble%20Tea-9b51e0">
  <a href="README.md"><img alt="en" src="https://img.shields.io/badge/lang-English-9b51e0"></a>
</p>

<p align="center">
  Если agyX был полезен — поставь репозиторию <b>★ звезду</b>.
</p>

---

<p align="center">
  <img src="assets/demo.gif" alt="agyX demo" width="900">
</p>

`agy` держит один залогиненный Google-аккаунт. agyX сохраняет каждый аккаунт
один раз в зашифрованный vault, показывает живую квоту всех аккаунтов рядом и
подменяет активный перед запуском `agy` — и для CLI, и для Antigravity IDE.

## Возможности

- **Квота**
- **CLI + IDE**
- **Патч region lock**
- **Зашифрованный vault**
- **Русский / английский**

## Установка

Скачай `agyx.exe` из [Releases](../../releases).

```powershell
.\agyx.exe            # портативный, работает из любого места
.\agyx.exe install    # или добавить в PATH (%LOCALAPPDATA%\Programs\agyx)
```

`agyx uninstall` убирает копию и запись в PATH; `~/.agyx` остаётся.

<details>
<summary>Сборка из исходников</summary>

Go 1.24+.

```powershell
git clone https://github.com/dycops/agyx.git
cd agyx
.\build.ps1            # версия берётся из последнего git-тега
                       # из Git Bash: powershell -File ./build.ps1
.\agyx.exe install
```

Простой `go build -o agyx.exe .` тоже работает — версия будет `dev`.
</details>

## Быстрый старт

```
agyx setup-oauth   # разово: извлечь OAuth-клиент в ~/.agyx/oauth.json
agyx add           # войти в Google-аккаунт через браузер
agyx add           # добавить ещё один (браузер даст выбрать)
agyx               # выбрать аккаунт, переключиться, запустить agy
```

`setup-oauth` читает OAuth-клиент Antigravity из локальной установки (Manager,
IDE или `agy.exe`) и пишет его в `oauth.json`. Файл переносимый: скопируй на
другую машину — там `agyx add` заработает при одном установленном `agy`.

## Клавиши в пикере

| клавиша | действие |
|---|---|
| `↑` `↓` / `j` `k` | навигация |
| `enter` | переключиться на аккаунт и запустить `agy` (или выполнить пункт) |
| `r` | обновить квоту в обход кэша |
| `d` | удалить выбранный аккаунт (`y` — подтвердить) |
| `q` `esc` | выход |

## Команды

```
agyx                 выбрать аккаунт и запустить agy с аргументами по умолчанию
agyx <agy args...>   то же, доп. аргументы добавляются для agy
agyx --plain [...]   запустить agy без аргументов по умолчанию (-p)
agyx add [метка]     войти в новый аккаунт через браузер и сохранить
agyx import [метка]  сохранить аккаунт, который сейчас в слоте
agyx list            список сохранённых аккаунтов
agyx rm <email>      удалить аккаунт из vault
agyx current         показать активный аккаунт
agyx quota           квота активного аккаунта по группам и окнам
agyx ide             записать активный аккаунт в Antigravity IDE (IDE закрыта)
agyx patch           патч region lock: остановить Antigravity, задать CLOUD_CODE_URL, пропатчить бинарники
agyx config          показать настройки;  agyx config args <...> — задать аргументы agy
agyx open-config     открыть config.json в $EDITOR / Notepad
agyx setup-oauth     разово извлечь OAuth-клиент в oauth.json
agyx install         скопировать agyx в %LOCALAPPDATA%\Programs\agyx и добавить в PATH
agyx uninstall       убрать копию и запись в PATH (vault остаётся)
agyx version         показать версию
```

## Настройки

`~/.agyx/config.json` создаётся при первом запуске:

```json
{
  "agy_args": ["--dangerously-skip-permissions"],
  "quota_ttl_sec": 60,
  "switch_ide": true,
  "auto_patch": false,
  "agy_autoupdate": false
}
```

| ключ | смысл |
|---|---|
| `agy_args` | аргументы, добавляемые к каждому запуску `agy`; `[]` — никаких |
| `quota_ttl_sec` | сколько держать квоту в кэше; `0` — запрашивать всегда |
| `switch_ide` | при свитче писать аккаунт и в Antigravity IDE (IDE должна быть закрыта, иначе пропуск с пометкой) |
| `auto_patch` | перед каждым запуском переприменять патч к `agy.exe`, если его заменило обновление |
| `agy_autoupdate` | `false` — запускать `agy` с `AGY_CLI_DISABLE_AUTO_UPDATE=1`, чтобы пропатченный бинарь не перезаписался; `true` — разрешить обновления |

Язык берётся из `AGYX_LANG`, затем `LC_ALL`, `LC_MESSAGES`, `LANG` — всё, что
начинается с `ru`, включает русский.

```powershell
$env:AGYX_LANG = "ru"; agyx
```

## Как это работает

- **Vault** — `~/.agyx/vault.bin`, JSON, зашифрованный Windows DPAPI
  (привязан к пользователю). Запись на аккаунт: email, значение креденшла ровно
  как его записал `agy`, время.
- **Свитч CLI** — запись Credential Manager `gemini` / `antigravity`
  перезаписывается через go-keyring, той же библиотекой, что у `agy`, поэтому
  значение проходит без изменений. Перед уходом с аккаунта его живое значение
  снимается обратно в vault — обновлённые токены не теряются.
- **Свитч IDE** — в `%APPDATA%\Antigravity IDE\User\globalStorage\state.vscdb`
  перезаписываются `antigravityUnifiedStateSync.oauthToken` / `userStatus` в
  protobuf-конверте IDE (то, что делает Antigravity Manager); разовый бэкап
  `state.vscdb.agyx.bak`.
- **Квота** — `cloudcode-pa.googleapis.com` `loadCodeAssist` +
  `retrieveUserQuotaSummary` с User-Agent Antigravity. В кэш (`quota.json`)
  попадают только проценты и время сброса, токены — никогда.
- **OAuth** — Authorization Code flow на loopback-порту `127.0.0.1` с публичным
  client id Antigravity и его app secret из `oauth.json`.

## Файлы

```
~/.agyx/
  vault.bin     DPAPI-зашифрованный vault аккаунтов
  oauth.json    OAuth client id + secret (креденшл приложения, переносимый)
  config.json   настройки
  quota.json    кэш квоты (только проценты и время сброса)
```

## Поддержать

Если agyX был полезен — поставь звезду. Баги и идеи — в Issues.

## Лицензия

MIT
