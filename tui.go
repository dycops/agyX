package main

// A width-aware bubbletea picker, drawn on the alternate screen so nothing is
// left behind in the terminal. One row per account: active marker, email,
// then two quota columns (Gemini, Claude/GPT), each holding a 5h cell and a
// weekly cell — reset countdown, thin bar, percent. Quota is fetched in the
// background with a spinner. Below the accounts: a divider, action rows, a
// log panel for actions that run inside the picker (patch, config), and a
// footer.

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Picker choices returned to the caller; account rows return the email.
const (
	chooseAdd    = "__add__"
	chooseImport = "__import__"
)

type action struct {
	icon, key, value string
}

// actions in display order; the last two run inside the picker.
var actions = []action{
	{"+", "act_add", chooseAdd},
	{"↧", "act_impt", chooseImport},
	{"#", "act_patch", "__patch__"},
	{"≡", "act_cfg", "__config__"},
}

const (
	barW     = 6         // cells in a bar
	pctW     = 4         // "NNN%"
	cellGap  = 3         // gap between the 5h and week cells
	colGap   = 4         // gap between the two group columns
	prefixW  = 2 + 1 + 1 // cursor(2) + dot(1) + space(1)
	emailGap = 2
	maxLog   = 8 // log lines kept in the panel
)

// timerW fits the longest countdown ("4d20h", "59m", "soon"/"скоро").
const timerW = 5

// cellW is one gauge: "4d20h ━━━━━━ NNN%".
const cellW = timerW + 1 + barW + 1 + pctW

// colW is one group's column: two cells (5h, week) and the gap between them.
const colW = cellW + cellGap + cellW

var (
	cAccent = lipgloss.Color("111")
	cGreen  = lipgloss.Color("78")
	cAmber  = lipgloss.Color("214")
	cRed    = lipgloss.Color("203")
	cDim    = lipgloss.Color("245")
	cFaint  = lipgloss.Color("238")

	titleSty  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).Background(lipgloss.Color("62")).Padding(0, 1)
	subSty    = lipgloss.NewStyle().Foreground(cDim)
	dotSty    = lipgloss.NewStyle().Foreground(cGreen)
	offDotSty = lipgloss.NewStyle().Foreground(cFaint)
	actionSty = lipgloss.NewStyle().Foreground(cAccent)
	trackSty  = lipgloss.NewStyle().Foreground(cFaint)
	headSty   = lipgloss.NewStyle().Foreground(cDim)
	quotaDim  = lipgloss.NewStyle().Foreground(cFaint)
	resetSty  = lipgloss.NewStyle().Foreground(cDim)
	logSty    = lipgloss.NewStyle().Foreground(cDim)
	cardSty   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("60")).Padding(1, 2)
	keySty    = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	keyDescSt = lipgloss.NewStyle().Foreground(cDim)
	sepSty    = lipgloss.NewStyle().Foreground(cFaint)
)

// Messages from background work.
type quotasMsg map[string]quotaInfo
type logMsg []string

type pickerModel struct {
	accts  []Account
	live   string
	quotas map[string]quotaInfo
	spin   spinner.Model

	loading    bool   // quota fetch in flight
	busy       string // label of the action running inside the picker, "" if none
	confirmDel int    // account row awaiting y/n for deletion, -1 if none
	log        []string

	cursor int
	chosen string
	width  int
}

func newPicker(accts []Account, live string) pickerModel {
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(lipgloss.NewStyle().Foreground(cAccent)))
	m := pickerModel{accts: accts, live: live, quotas: map[string]quotaInfo{}, spin: sp, loading: true, confirmDel: -1}
	return m
}

func (m pickerModel) Init() tea.Cmd { return m.fetchQuotas(false) }

// fetchQuotas starts the spinner and loads quota in the background; force
// bypasses the cache.
func (m *pickerModel) fetchQuotas(force bool) tea.Cmd {
	m.loading = true
	accts := append([]Account(nil), m.accts...)
	return tea.Batch(m.spin.Tick, func() tea.Msg { return quotasMsg(prefetchQuotas(accts, force)) })
}

// deleteAccount removes the row from the vault and the picker.
func (m *pickerModel) deleteAccount(i int) {
	email := m.accts[i].Email
	m.accts = append(m.accts[:i], m.accts[i+1:]...)
	delete(m.quotas, email)
	if err := saveVault(m.accts); err != nil {
		m.log = append(m.log, fmt.Sprintf(T("del_err"), err))
	} else {
		m.log = append(m.log, fmt.Sprintf(T("del_done"), email))
	}
	if m.cursor >= m.rowCount() {
		m.cursor = m.rowCount() - 1
	}
}

func (m pickerModel) rowCount() int { return len(m.accts) + len(actions) }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case spinner.TickMsg:
		if !m.loading && m.busy == "" {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case quotasMsg:
		m.quotas, m.loading = msg, false
		m.sortAccounts()
	case logMsg: // an in-picker action finished and reports its lines
		m.busy = ""
		m.log = append(m.log, msg...)
		if len(m.log) > maxLog {
			m.log = m.log[len(m.log)-maxLog:]
		}
	case tea.KeyMsg:
		if m.busy != "" {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}
		if m.confirmDel >= 0 {
			// Pending "delete? y/n": only y confirms, anything else cancels.
			if msg.String() == "y" {
				m.deleteAccount(m.confirmDel)
			}
			m.confirmDel = -1
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "enter":
			return m.choose()
		case "d":
			if m.cursor < len(m.accts) {
				m.confirmDel = m.cursor
			}
		case "r":
			if !m.loading {
				return m, m.fetchQuotas(true)
			}
		case "q", "esc", "ctrl+c":
			m.chosen = ""
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *pickerModel) moveCursor(d int) {
	n := m.rowCount()
	m.cursor = (m.cursor + d + n) % n
}

// sortAccounts orders by Gemini 5h remaining ascending, then weekly; unknown
// quota (-1) first.
func (m *pickerModel) sortAccounts() {
	q := m.quotas
	sort.SliceStable(m.accts, func(i, j int) bool {
		a, b := q[m.accts[i].Email].gem, q[m.accts[j].Email].gem
		if a.fiveHPct != b.fiveHPct {
			return a.fiveHPct < b.fiveHPct
		}
		return a.pct < b.pct
	})
}

// choose acts on the cursor row: accounts and add/import leave the picker,
// patch and config run inside it and report to the log panel.
func (m pickerModel) choose() (tea.Model, tea.Cmd) {
	if m.cursor < len(m.accts) {
		m.chosen = m.accts[m.cursor].Email
		return m, tea.Quit
	}
	act := actions[m.cursor-len(m.accts)]
	switch act.value {
	case "__patch__":
		m.busy = T("patch_busy")
		return m, tea.Batch(m.spin.Tick, func() tea.Msg {
			var lines []string
			runPatch(func(s string) { lines = append(lines, s) })
			return logMsg(lines)
		})
	case "__config__":
		cmd, err := editorCmd()
		if err != nil {
			return m, func() tea.Msg { return logMsg{fmt.Sprintf(T("cfg_err"), err)} }
		}
		m.busy = T("cfg_busy")
		// ExecProcess hands the terminal to the editor and restores the
		// picker when it exits (works for console editors and Notepad alike).
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				return logMsg{fmt.Sprintf(T("cfg_err"), err)}
			}
			return logMsg{T("cfg_saved")}
		})
	default:
		m.chosen = act.value
		return m, tea.Quit
	}
}

func quotaColorByPct(p int) lipgloss.Color {
	switch {
	case p >= 50:
		return cGreen
	case p >= 20:
		return cAmber
	default:
		return cRed
	}
}

// pct renders "NNN%" colored by level, or a dim placeholder when unknown.
func pct(p int) string {
	if p < 0 {
		return quotaDim.Render(fmt.Sprintf("%*s", pctW, "·"))
	}
	return lipgloss.NewStyle().Foreground(quotaColorByPct(p)).Bold(true).Render(fmt.Sprintf("%*d%%", pctW-1, p))
}

// bar renders a thin bar: "━━━━──" colored by level over a faint solid track.
func bar(p int) string {
	if p < 0 {
		return quotaDim.Render(strings.Repeat("─", barW))
	}
	filled := (p*barW + 50) / 100
	if filled > barW {
		filled = barW
	}
	col := lipgloss.NewStyle().Foreground(quotaColorByPct(p))
	return col.Render(strings.Repeat("━", filled)) + trackSty.Render(strings.Repeat("─", barW-filled))
}

// cell renders one gauge with its reset countdown in front: "4d20h ━━━━──  29%".
func cell(p int, reset string) string {
	d := shortDur(reset)
	if d == "" {
		d = "–"
	}
	t := resetSty.Render(fmt.Sprintf("%*s", timerW, d))
	return t + " " + bar(p) + " " + pct(p)
}

// quotaCol renders one group's column: the 5h cell, then the week cell.
func quotaCol(g groupQuota) string {
	return cell(g.fiveHPct, g.fiveHReset) + strings.Repeat(" ", cellGap) + cell(g.pct, g.weekReset)
}

// shortDur formats the time until an RFC3339 instant, compactly and localized.
func shortDur(rfc string) string {
	if rfc == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, rfc)
	if err != nil {
		return ""
	}
	d := time.Until(t)
	if d <= 0 {
		return T("reset_soon")
	}
	days := int(d.Hours()) / 24
	hrs := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%d%s%d%s", days, T("u_d"), hrs, T("u_h"))
	case hrs > 0:
		return fmt.Sprintf("%d%s%d%s", hrs, T("u_h"), mins, T("u_m"))
	default:
		return fmt.Sprintf("%d%s", mins, T("u_m"))
	}
}

// truncEmail cuts s to w display cells (labels may be non-ASCII).
func truncEmail(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	if w <= 1 {
		return string(r[:w])
	}
	return string(r[:w-1]) + "…"
}

// pad right-pads s with spaces to visible width w.
func pad(s string, w int) string {
	return s + strings.Repeat(" ", max(0, w-lipgloss.Width(s)))
}

func (m pickerModel) View() string {
	emailW := 8
	for _, a := range m.accts {
		if w := lipgloss.Width(a.Email); w > emailW {
			emailW = w
		}
	}
	if emailW > 30 {
		emailW = 30
	}
	avail := m.width
	if avail <= 0 {
		avail = 100
	}
	avail -= 8 // card border + padding
	quotasW := colW + colGap + colW
	if need := prefixW + emailW + emailGap + quotasW; need > avail {
		if emailW = avail - prefixW - emailGap - quotasW; emailW < 10 {
			emailW = 10
		}
	}
	rowW := prefixW + emailW + emailGap + quotasW
	gap := strings.Repeat(" ", colGap)
	cursorAt := func(i int) string {
		if i == m.cursor {
			return keySty.Render("▎") + " "
		}
		return "  "
	}

	var b strings.Builder
	b.WriteString(titleSty.Render(" agyX ") + " " + subSty.Render(T("tagline")) + "  " + quotaDim.Render("v"+version) + "\n\n")

	// Header: group names over their columns (cells are 5h, then week).
	b.WriteString(strings.Repeat(" ", prefixW) + pad(headSty.Render(T("h_account")), emailW+emailGap) +
		pad(headSty.Render("Gemini"), colW) + gap + headSty.Render("Claude / GPT") + "\n")

	for i, a := range m.accts {
		dot := offDotSty.Render("○")
		if sameAccount(a.Token, m.live) {
			dot = dotSty.Render("●")
		}
		emailSty := lipgloss.NewStyle().Width(emailW).Foreground(cDim)
		if i == m.cursor {
			emailSty = emailSty.Foreground(lipgloss.Color("231")).Bold(true)
		}
		if i == m.confirmDel {
			emailSty = emailSty.Foreground(cRed).Bold(true)
		}
		email := emailSty.Render(truncEmail(a.Email, emailW))
		var quota string
		switch {
		case i == m.confirmDel:
			quota = lipgloss.NewStyle().Foreground(cRed).Render(T("del_ask")) + " " + keySty.Render("y") + keyDescSt.Render("/") + keySty.Render("n")
		case m.loading:
			quota = m.spin.View() + " " + subSty.Render(T("loading"))
		default:
			qi := m.quotas[a.Email]
			quota = quotaCol(qi.gem) + gap + quotaCol(qi.cla)
		}
		b.WriteString(cursorAt(i) + dot + " " + email + strings.Repeat(" ", emailGap) + quota + "\n")
	}

	b.WriteString(sepSty.Render(strings.Repeat("─", rowW)) + "\n")
	for j, act := range actions {
		i := len(m.accts) + j
		b.WriteString(cursorAt(i) + actionSty.Render(act.icon) + " " + actionSty.Render(T(act.key)) + "\n")
	}

	// Log panel: only when something ran inside the picker.
	if m.busy != "" || len(m.log) > 0 {
		b.WriteString(sepSty.Render(strings.Repeat("─", rowW)) + "\n")
		for _, l := range m.log {
			b.WriteString("  " + logSty.Render(truncEmail(l, rowW-2)) + "\n")
		}
		if m.busy != "" {
			b.WriteString("  " + m.spin.View() + " " + subSty.Render(m.busy) + "\n")
		}
	}

	sep := sepSty.Render("  ·  ")
	keys := keySty.Render("↑↓") + " " + keyDescSt.Render(T("k_select")) + sep +
		keySty.Render("enter") + " " + keyDescSt.Render(T("k_run")) + sep +
		keySty.Render("r") + " " + keyDescSt.Render(T("k_refresh")) + sep +
		keySty.Render("d") + " " + keyDescSt.Render(T("k_delete")) + sep +
		keySty.Render("q") + " " + keyDescSt.Render(T("k_quit"))

	return cardSty.Render(b.String()+"\n"+keys) + "\n"
}

// runPicker shows the picker and returns the chosen account email or action
// ("" if the user quit).
func runPicker(accts []Account, live string) (string, error) {
	m, err := tea.NewProgram(newPicker(accts, live), tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}
	return m.(pickerModel).chosen, nil
}
