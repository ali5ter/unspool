package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/classifier"
	"github.com/ali5ter/unspool/internal/store"
)

// startInspect handles the `i` key (PRD §5.2.4 tier 2, on-demand): shows a
// cached verdict immediately if one exists, otherwise dispatches an async
// shell-out to cfg.Classifier.InspectCommand. A no-op (just a status
// message) when no inspect command is configured — this is opt-in and
// model-agnostic by design, not something unspool ships a default for.
func (m Model) startInspect() (bool, Model, tea.Cmd) {
	video, _, ok := m.selectedVideo()
	if !ok {
		return true, m, nil
	}
	if m.cfg.Classifier.InspectCommand == "" {
		m.statusMsg = "inspect: no classifier.inspect_command configured"
		return true, m, nil
	}
	if m.cfg.Classifier.CacheVerdicts {
		if vf, err := m.store.LoadVerdicts(); err == nil {
			if v, ok := vf.Verdicts[video.VideoID]; ok {
				m.inspecting = true
				m.inspectResult = v
				m.inspectErr = nil
				return true, m, clearScreenCmd()
			}
		}
	}
	m.inspectVideoID = video.VideoID
	m.statusMsg = "inspecting…"
	m.busy = true
	return true, m, tea.Batch(inspectCmd(m.cfg, m.store, video), m.spinner.Tick)
}

// inspectDoneMsg carries the result of an inspectCmd back to the model.
// videoID is the staleness-guard, compared against m.inspectVideoID at
// receipt — same shape as playlistItemsLoadedMsg's playlistID guard.
type inspectDoneMsg struct {
	videoID string
	verdict store.Verdict
	err     error
}

func inspectCmd(cfg *config.Config, st *store.Store, v store.Video) tea.Cmd {
	return func() tea.Msg {
		url := "https://www.youtube.com/watch?v=" + v.VideoID
		cv, err := classifier.RunInspect(context.Background(), cfg.Classifier.InspectCommand, url)
		if err != nil {
			return inspectDoneMsg{videoID: v.VideoID, err: err}
		}
		sv := store.Verdict{
			Score:          cv.Score,
			LikelyAI:       cv.LikelyAI,
			Reasoning:      cv.Reasoning,
			SuspectedTools: cv.SuspectedTools,
		}
		if cfg.Classifier.CacheVerdicts {
			_ = st.SetVerdict(v.VideoID, sv)
		}
		return inspectDoneMsg{videoID: v.VideoID, verdict: sv}
	}
}

func (m Model) handleInspectDone(msg inspectDoneMsg) (tea.Model, tea.Cmd) {
	if msg.videoID != m.inspectVideoID {
		return m, nil // stale — user has since moved on
	}
	m.busy = false
	if msg.err != nil {
		m.statusMsg = "inspect failed: " + firstLine(msg.err.Error())
		return m, nil
	}
	m.inspecting = true
	m.inspectResult = msg.verdict
	m.inspectErr = nil
	m.statusMsg = "inspected"
	if m.verdicts == nil {
		m.verdicts = map[string]store.Verdict{}
	}
	m.verdicts[msg.videoID] = msg.verdict
	if msg.verdict.LikelyAI {
		// Badge whatever list this was inspected from immediately, rather
		// than leaving it stale until that list's next reload — the one
		// moment a user is actually watching for this is right after
		// pressing `i`.
		m.patchInspectedBadge(msg.videoID)
	}
	return m, clearScreenCmd()
}

func (m Model) updateInspecting(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Back) || key.Matches(msg, m.keys.Confirm) {
		m.inspecting = false
		return m, clearScreenCmd()
	}
	return m, nil
}

// inspectDialogMaxWidth caps how wide the inspect dialog's free-text
// content (an LLM's own reasoning, which can run to hundreds of characters
// on a single unwrapped "line" with no natural break points) is allowed to
// wrap to — renderDialog sizes the whole dialog box to fit its longest
// line, so without a cap here a long-winded verdict could render a dialog
// far wider than the terminal itself. Confirmed live: an unwrapped verdict
// produced a dialog wide enough to overflow the terminal entirely.
const inspectDialogMaxWidth = 70

// renderInspect shows the tier-2 verdict — explicit advisory language
// throughout, per PRD §5.2's "never assert certainty" principle.
func (m Model) renderInspect() string {
	v := m.inspectResult
	verdict := "not flagged as likely AI"
	if v.LikelyAI {
		verdict = "likely AI-generated"
	}

	w := m.width - 10
	if w > inspectDialogMaxWidth {
		w = inspectDialogMaxWidth
	}
	if w < 20 {
		w = 20
	}
	wrap := lipgloss.NewStyle().Width(w)

	lines := []string{styleMeta.Render(verdict)}
	if v.Score != nil {
		lines = append(lines, styleMeta.Render(fmt.Sprintf("score: %.2f", *v.Score)))
	}
	if v.Reasoning != "" {
		lines = append(lines, "", wrap.Render(v.Reasoning))
	}
	if len(v.SuspectedTools) > 0 {
		lines = append(lines, "", wrap.Render(styleMeta.Render("suspected tools: "+strings.Join(v.SuspectedTools, ", "))))
	}
	lines = append(lines, "", styleMeta.Render("Advisory only — not a verified fact."))
	body := strings.Join(lines, "\n")
	return renderDialog("Inspect (advisory)", body, "esc close")
}
