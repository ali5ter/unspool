package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ali5ter/unspool/internal/auth"
)

// setupScriptPath is scripts/setup-gcp.sh's path relative to the working
// directory unspool is launched from — the only place it can live, since a
// Homebrew/`go install` binary doesn't carry the repo's scripts/ directory
// with it (see findSetupScript).
const setupScriptRelPath = "scripts/setup-gcp.sh"

// findSetupScript returns setupScriptRelPath if it's present on disk, or ""
// otherwise. Only true when unspool is run from within a clone of its own
// repo (`go run .` / a from-source build) — gates whether the setup screen
// offers to run it directly (see viewSetupNeeded/updateSetupNeeded) versus
// just pointing at docs/SETUP.md.
func findSetupScript() string {
	if info, err := os.Stat(setupScriptRelPath); err == nil && !info.IsDir() {
		return setupScriptRelPath
	}
	return ""
}

// setupScriptDoneMsg carries the result of running scripts/setup-gcp.sh via
// tea.ExecProcess back into the model.
type setupScriptDoneMsg struct{ err error }

// updateSetupNeeded handles key presses on the setup screen (m.needsSetup).
// Quit is already handled unconditionally upstream in updateInner.
func (m Model) updateSetupNeeded(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.settingUp {
		return m, nil
	}
	switch msg.String() {
	case "s":
		if m.setupScriptPath == "" {
			return m, nil
		}
		m.settingUp = true
		m.setupErr = nil
		// tea.ExecProcess suspends the Bubble Tea renderer and hands the
		// real terminal to the child for the duration of the call — the
		// script is fully interactive (gcloud auth, pfb confirm prompts,
		// opens a browser for the OAuth consent screen), so it can't run
		// detached the way mpv does.
		cmd := exec.Command(m.setupScriptPath)
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return setupScriptDoneMsg{err: err}
		})
	case "r":
		return m.recheckSetup()
	}
	return m, nil
}

// recheckSetup re-stats cfg.OAuthClientSecretFile — used after the setup
// script exits and by its own manual "r" recheck key, since setup-gcp.sh
// only covers the GCP-project/API-enable step; the OAuth consent screen
// and client creation are manual (Google has no scriptable path for
// either — see setup-gcp.sh), so success can't be assumed from the script
// alone. Falls through into the login flow (if no token is stored yet —
// see issue #11) or straight into sync once the file exists.
func (m Model) recheckSetup() (tea.Model, tea.Cmd) {
	if _, err := os.Stat(m.cfg.OAuthClientSecretFile); err != nil {
		m.setupErr = fmt.Errorf("still not found at the path above")
		return m, nil
	}
	m.needsSetup = false
	m.setupErr = nil
	if !auth.HasStoredToken() {
		m.needsLogin = true
		m.loggingIn = true
		return m, tea.Batch(loginProcessCmd(), clearScreenCmd())
	}
	m.syncing = true
	m.busy = true
	m.statusMsg = "syncing…"
	return m, tea.Batch(m.spinner.Tick, runSync(m.cfg))
}

// handleSetupScriptDone applies the result of running setup-gcp.sh and
// recheck's for the client secret file — see recheckSetup. clearScreenCmd
// is required here (not just cosmetic): ExecProcess handed the whole
// terminal to the script for however long it ran, so whatever's on screen
// now is stale and needs a full repaint either way, success or failure.
func (m Model) handleSetupScriptDone(msg setupScriptDoneMsg) (tea.Model, tea.Cmd) {
	m.settingUp = false
	if msg.err != nil {
		m.setupErr = fmt.Errorf("setup script: %w", msg.err)
		return m, clearScreenCmd()
	}
	next, cmd := m.recheckSetup()
	return next, tea.Batch(clearScreenCmd(), cmd)
}

// viewSetupNeeded renders the startup screen shown instead of the sync
// splash when cfg.OAuthClientSecretFile is missing — walks through
// docs/SETUP.md's steps in place, with an option to run scripts/
// setup-gcp.sh's scriptable half directly, rather than letting sync fail
// and surface an opaque "can't find client_secret.json" error in the
// footer after the fact (issue #9).
func (m Model) viewSetupNeeded() string {
	lines := []string{
		"unspool talks to YouTube as you, via your own free Google Cloud",
		"OAuth client (docs/SETUP.md) — none was found. Expected path:",
		"",
		styleTitle.Render("  " + m.cfg.OAuthClientSecretFile),
		"",
		styleMeta.Render("  1. Create a GCP project and enable the YouTube Data API v3"),
		styleMeta.Render("  2. Configure the OAuth consent screen (External; add yourself"),
		styleMeta.Render("     as a test user)"),
		styleMeta.Render("  3. Create an OAuth client ID (Desktop app) and download its JSON"),
		styleMeta.Render("  4. Save it to the path above"),
		styleMeta.Render("  5. unspool logs you in automatically once this file is in place"),
	}
	if m.setupScriptPath != "" {
		lines = append(lines, "", styleMeta.Render("'s' runs step 1 (scripts/setup-gcp.sh) for you; 2-3 have no"), styleMeta.Render("scriptable path and stay manual."))
	}
	if m.setupErr != nil {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render(firstLine(m.setupErr.Error())))
	}
	body := strings.Join(lines, "\n")

	hint := "r recheck   ctrl+c quit"
	if m.setupScriptPath != "" {
		hint = "s run setup script   r recheck   ctrl+c quit"
	}
	dialog := renderDialogNoTitle(body, hint)
	content := lipgloss.JoinVertical(lipgloss.Center, renderSplashLogoSweep(m.pulseTick), "", dialog)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// loginDoneMsg carries the result of running the interactive OAuth login
// flow (via a re-exec'd "unspool --login", handed the terminal through
// tea.ExecProcess — same shape as setupScriptDoneMsg) back into the model.
type loginDoneMsg struct{ err error }

// loginProcessCmd re-execs the running binary as "unspool --login" and
// hands it the terminal via tea.ExecProcess — auth.Login itself prints
// interactive prompts (the consent URL, a "close this tab" notice) and
// opens a browser, which needs the real terminal, not bubbletea's
// managed one. Re-exec'ing rather than calling auth.Login directly here
// keeps that terminal-taking behavior scoped to cmd/login.go's existing,
// already-correct entry point instead of duplicating it in internal/tui.
func loginProcessCmd() tea.Cmd {
	exe, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return loginDoneMsg{err: fmt.Errorf("locate unspool executable: %w", err)} }
	}
	cmd := exec.Command(exe, "--login")
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return loginDoneMsg{err: err}
	})
}

// handleLoginDone applies the result of the login subprocess. Success
// falls straight through into the normal sync path (same as recheckSetup
// falling through once a token already exists); failure clears
// m.loggingIn and leaves m.needsLogin set so viewLoginNeeded shows the
// error with a manual "r" retry — deliberately not auto-retried, so a
// closed browser tab or timed-out loopback callback (see auth.Login)
// doesn't loop the login prompt back up instantly.
func (m Model) handleLoginDone(msg loginDoneMsg) (tea.Model, tea.Cmd) {
	m.loggingIn = false
	if msg.err != nil {
		m.loginErr = fmt.Errorf("login: %w", msg.err)
		return m, clearScreenCmd()
	}
	m.needsLogin = false
	m.loginErr = nil
	m.syncing = true
	m.busy = true
	m.statusMsg = "syncing…"
	return m, tea.Batch(clearScreenCmd(), m.spinner.Tick, runSync(m.cfg))
}

// updateLoginNeeded handles key presses on the login screen (m.needsLogin
// with m.loggingIn false, i.e. a previous attempt failed — see
// handleLoginDone). Quit is already handled unconditionally upstream in
// updateInner.
func (m Model) updateLoginNeeded(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.loggingIn {
		return m, nil
	}
	if msg.String() == "r" {
		m.loggingIn = true
		m.loginErr = nil
		return m, tea.Batch(loginProcessCmd(), clearScreenCmd())
	}
	return m, nil
}

// viewLoginNeeded renders the screen shown once the OAuth client secret
// is in place but no token is stored yet (m.needsLogin — issue #11). The
// happy path never lingers here: loginProcessCmd fires from Init/
// recheckSetup and immediately hands the terminal to "unspool --login"
// via tea.ExecProcess, so this is only actually seen for the brief
// instant before that handoff, or after a failed attempt (m.loginErr
// set), where it offers a manual "r" retry instead of looping
// automatically.
func (m Model) viewLoginNeeded() string {
	lines := []string{
		"unspool needs to log in to YouTube as you — this only happens once.",
	}
	if m.loggingIn {
		lines = append(lines, "", styleMeta.Render("  Opening your browser for the Google consent screen…"))
	} else {
		lines = append(lines, "", styleMeta.Render("  Handing off to 'unspool --login' opens your browser for the"), styleMeta.Render("  Google consent screen, then stores a refresh token in your"), styleMeta.Render("  system keychain."))
	}
	if m.loginErr != nil {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render(firstLine(m.loginErr.Error())))
	}
	body := strings.Join(lines, "\n")

	hint := "logging in…"
	if !m.loggingIn {
		hint = "r retry   ctrl+c quit"
	}
	dialog := renderDialogNoTitle(body, hint)
	content := lipgloss.JoinVertical(lipgloss.Center, renderSplashLogoSweep(m.pulseTick), "", dialog)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
