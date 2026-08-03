// SPDX-License-Identifier: GPL-3.0-only

// Package tui implements a chat surface on top of the core agent loop. It reads user
// input and starts a run in a goroutine. Rather than taking exclusive ownership of the
// terminal, chat content is printed as ordinary output so the terminal scrolls it into
// its normal scrollback; only a fixed footer (spinner, context summary, input, help)
// is kept pinned at the bottom of the screen. The package is intentionally thin: it
// owns presentation and input only, never the loop itself.
package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"thing/internal/agent"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
	youStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	agentStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	toolStyle  = lipgloss.NewStyle().Faint(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	usageStyle = lipgloss.NewStyle().Faint(true)
)

// runFinishedMsg reports that the run with the given id has ended (normally or by
// cancellation). It carries the run id so a stale message from a cancelled run can
// never clobber the state of a newer one.
type runFinishedMsg struct{ id int }

// keyMap describes the bindings surfaced in the help line. Enter and Esc are also
// handled directly; the struct exists so the help component can describe them.
type keyMap struct {
	Send    key.Binding
	Cancel  key.Binding
	History key.Binding
	Quit    key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Send:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send")),
		Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "interrupt")),
		History: key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "history")),
		Quit:    key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
	}
}

// Model is the top-level Bubble Tea model for the chat UI.
type Model struct {
	agent *agent.Agent
	app   *tea.Program

	running bool
	cancel  context.CancelFunc
	runID   int

	input   textinput.Model
	mention mention
	skill   skillPopup
	history []string
	histIdx int // -1 means a fresh (empty) input; otherwise an index into history
	spinner spinner.Model
	help    help.Model
	keys    keyMap

	width, height int
}

func newModel(a *agent.Agent) *Model {
	input := textinput.New()
	input.Placeholder = "Message the agent…"
	input.Prompt = "> "
	input.CharLimit = 0
	input.Focus()

	return &Model{
		agent:   a,
		input:   input,
		mention: newMention(),
		skill:   newSkillPopup(),
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		help:    help.New(),
		keys:    defaultKeyMap(),
		histIdx: -1,
	}
}

// New builds the chat program (wired to the given agent) and returns it. The entry
// point runs it with (*tea.Program).Run. Esc interrupts an in-flight request;
// Ctrl-C quits and cancels anything in flight.
func New(a *agent.Agent) *tea.Program {
	m := newModel(a)
	app := tea.NewProgram(m)
	m.app = app
	return app
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tea.Println(titleStyle.Render("Thing")))
}

func (m *Model) storeHistory(s string) {
	m.history = append(m.history, s)
	m.histIdx = -1
}

func (m *Model) cancelRun() {
	if m.cancel != nil {
		m.cancel()
	}
	m.running = false
	m.cancel = nil
}

// renderMarkdown renders markdown to a set of ANSI-styled lines for printing into the
// terminal scrollback, falling back to the literal text if the renderer fails.
func renderMarkdown(s string) []string {
	out, err := glamour.Render(s, "dark")
	if err != nil {
		return strings.Split(s, "\n")
	}
	return strings.Split(strings.TrimSuffix(out, "\n"), "\n")
}

// labeled renders a styled label line followed by the markdown-rendered body.
func labeled(label string, style lipgloss.Style, body string) []string {
	lines := []string{style.Render(label)}
	lines = append(lines, renderMarkdown(body)...)
	return lines
}

// renderEvent turns a core Event into the styled line(s) printed into scrollback.
func renderEvent(ev agent.Event) []string {
	switch ev.Kind {
	case agent.KindAssistant:
		return labeled("Agent", agentStyle, ev.Message)
	case agent.KindFinal:
		return labeled("Agent", agentStyle, ev.Message)
	case agent.KindToolCall:
		return []string{toolStyle.Render("  ↳ " + ev.Tool + " " + ev.ToolInput), ""}
	case agent.KindToolResult:
		return []string{toolStyle.Render("  " + tailLines(ev.Message, 3)), ""}
	case agent.KindError:
		return []string{errorStyle.Render("error: " + ev.Message)}
	default:
		return nil
	}
}

// tailLines keeps the last n non-empty lines of s. It is used so a tool's full output
// (already fed back to the model in the conversation) does not spam the scrollback.
func tailLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	kept := make([]string, 0, n)
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		kept = append(kept, l)
		if len(kept) > n {
			kept = kept[1:]
		}
	}
	if len(kept) == 0 {
		return ""
	}
	if len(kept) < n {
		return strings.Join(kept, "\n")
	}
	return "… (truncated)\n" + strings.Join(kept, "\n")
}

// renderUser turns a submitted input into the echo lines printed into scrollback.
func renderUser(input string) []string {
	return labeled("You", youStyle, input)
}

func (m Model) helpBindings() []key.Binding {
	return []key.Binding{m.keys.Send, m.keys.Cancel, m.keys.History, m.keys.Quit}
}

// handleEnter starts a run for the current input. While a request is in flight Enter
// is ignored so runs never overlap (they would race on the agent's shared chat state);
// the input itself stays focused and typeable throughout.
func (m Model) handleEnter() (Model, tea.Cmd) {
	if m.running {
		return m, nil
	}
	trimmed := strings.TrimSpace(m.input.Value())
	m.input.SetValue("")
	if trimmed == "" {
		return m, nil
	}

	m.storeHistory(trimmed)

	// User-explicit skill invocation is a TUI concern: the interception syntax
	// ("/skill:<name> task") is recognized here, not in the core. The echo and
	// history keep the raw command; only the string actually sent to the model is
	// the resolved activation pointer. On an unknown skill we print the error into
	// the scrollback and do not start a run.
	sent, err := m.resolveInput(trimmed)
	if err != nil {
		m.runID++
		id := m.runID
		app := m.app
		go func() {
			for _, l := range renderUser(trimmed) {
				app.Println(l)
			}
			for _, l := range renderEvent(agent.Event{Kind: agent.KindError, Message: err.Error()}) {
				app.Println(l)
			}
			app.Send(runFinishedMsg{id: id})
		}()
		return m, func() tea.Msg { return m.spinner.Tick() }
	}

	m.runID++
	id := m.runID
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.running = true

	app := m.app
	events := m.agent.Run(ctx, sent)
	go func() {
		// Echo the user's line, then print each event, as ordinary terminal output.
		// Program.Println emits the line "above" the program: it persists in the
		// terminal's scrollback while the footer stays pinned at the bottom.
		for _, l := range renderUser(trimmed) {
			app.Println(l)
		}
		for ev := range events {
			for _, l := range renderEvent(ev) {
				app.Println(l)
			}
		}
		app.Send(runFinishedMsg{id: id})
	}()

	return m, func() tea.Msg { return m.spinner.Tick() }
}

// skillCommandPrefix is the manual-activation command form, e.g. "/skill:git-commit".
const skillCommandPrefix = "/skill:"

// parseSkillCommand reports whether input begins with "/skill:" and, if so, returns
// the skill name and any trailing task text after the name (empty when the command
// is exactly "/skill:<name>"). Parsing the shorthand belongs to the interaction
// surface; the core never sees keyboard shorthand.
func parseSkillCommand(input string) (name, task string, ok bool) {
	if !strings.HasPrefix(input, skillCommandPrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(input, skillCommandPrefix)
	if rest == "" {
		return "", "", true // "/skill:" with no name — caller resolves (will fail)
	}
	if i := strings.IndexAny(rest, " 	"); i >= 0 {
		name = rest[:i]
		task = strings.TrimSpace(rest[i:])
	} else {
		name = rest
	}
	return name, task, true
}

// resolveInput turns submitted input into the string that will actually be sent to
// the model. For a "/skill:<name> task" command it asks the core to activate the
// skill and returns the resolved pointer (an error if the skill is unknown); any
// other input passes through unchanged. The raw input is what gets echoed to the
// scrollback and stored in history; only this resolved string reaches the model.
func (m Model) resolveInput(input string) (string, error) {
	name, task, isCmd := parseSkillCommand(input)
	if !isCmd || m.agent == nil {
		return input, nil
	}
	return m.agent.ActivateSkill(name, task)
}

// navigateHistory moves through prior inputs on arrow up/down (readline-like).
func (m *Model) navigateHistory(direction string) {
	if len(m.history) == 0 {
		return
	}
	switch direction {
	case "up":
		switch {
		case m.histIdx < 0:
			m.histIdx = len(m.history) - 1
		case m.histIdx > 0:
			m.histIdx--
		default:
			return
		}
	case "down":
		switch {
		case m.histIdx < 0:
			return
		case m.histIdx == len(m.history)-1:
			m.histIdx = -1
			m.input.SetValue("")
			return
		default:
			m.histIdx++
		}
	}
	m.input.SetValue(m.history[m.histIdx])
	m.input.CursorEnd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = msg.Width - 2
		m.help.Width = msg.Width
		return m, nil

	case runFinishedMsg:
		if msg.id == m.runID {
			m.running = false
			m.cancel = nil
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		// A popup (file-mention or skill) is active: arrows move the selection,
		// Tab/Enter commit, Esc closes. All other keys edit the input and re-sync
		// the popups. The two popups are parallel state machines and mutually
		// exclusive as data sources, but a skill popup is impossible while a
		// mention is open (typing "/" clears the "@" query) and vice versa.
		if m.mention.open || m.skill.open {
			switch msg.String() {
			case "up":
				if m.skill.open {
					return m.skillCursorUp(), nil
				}
				return m.mentionCursorUp(), nil
			case "down":
				if m.skill.open {
					return m.skillCursorDown(), nil
				}
				return m.mentionCursorDown(), nil
			case "esc":
				if m.skill.open {
					return m.skillClosed(), nil
				}
				return m.mentionClosed(), nil
			case "tab", "enter", "right":
				if m.skill.open {
					return m.commitSkill(), nil
				}
				return m.commitMention(), nil
			}
		}
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cancelRun()
			return m, tea.Quit
		case key.Matches(msg, m.keys.Cancel):
			if m.running {
				m.cancelRun()
			} else if m.skill.open {
				return m.skillClosed(), nil
			} else if m.mention.open {
				return m.mentionClosed(), nil
			}
			return m, nil
		case key.Matches(msg, m.keys.Send):
			return m.handleEnter()
		case key.Matches(msg, m.keys.History):
			// Arrow up/down while NOT typing a popup is history navigation.
			if !m.mention.open && !m.skill.open {
				m.navigateHistory(msg.String())
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			m = m.refreshMention()
			m = m.refreshSkill()
			return m, cmd
		}

	default:
		return m, nil
	}
}

// View renders only the fixed footer pinned at the bottom of the terminal: spinner
// (when working), context summary, input, and help. Chat content is not part of the
// frame; it is printed into the terminal scrollback by handleEnter's goroutine.
func (m Model) View() string {
	var parts []string
	if m.running {
		parts = append(parts, m.spinner.View()+" working…")
	} else {
		parts = append(parts, "")
	}
	if m.mention.open {
		parts = append(parts, m.renderMention())
	}
	if m.skill.open {
		parts = append(parts, m.renderSkill())
	}
	if m.agent != nil {
		parts = append(parts, usageStyle.Render(m.usageSummary()))
	}
	parts = append(parts, m.input.View())
	parts = append(parts, m.help.ShortHelpView(m.helpBindings()))
	return strings.Join(parts, "\n")
}

// usageSummary reports the live context usage of the most recent model response for
// the context-summary line.
func (m *Model) usageSummary() string {
	u := m.agent.Usage()
	return fmt.Sprintf("─ ctx: %d in / %d out  cache: %.1f%% (%d/%d)",
		u.PromptTokens,
		u.CompletionTokens,
		u.CachedTokensRatio*100,
		u.CachedTokens,
		u.PromptTokens,
	)
}
