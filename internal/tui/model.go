// SPDX-License-Identifier: GPL-3.0-only

// Package tui provides a terminal user interface for the application.
package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"thing/internal/agent"
	thingmodel "thing/internal/model"
	"thing/internal/tui/statusbar"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
)

// runFinishedMsg reports that the current run has ended (normally or by cancellation).
type runFinishedMsg struct{}

// model stores the application state.
type model struct {
	// Dependencies
	p     *tea.Program
	agent *agent.Agent

	// State
	isWorking bool
	cancel    context.CancelFunc

	// Views
	spinner   spinner.Model
	textarea  textarea.Model
	statusbar statusbar.Model
	help      help.Model
	keys      keyMap
}

// initialModel returns the initial state of the application.
func initialModel(agent *agent.Agent) model {
	// "Working" spinner.
	s := spinner.New()
	s.Spinner = spinner.Meter
	s.Style = lipgloss.NewStyle().Foreground(primaryColor)

	// Input text area.
	t := textarea.New()
	t.DynamicHeight = true
	t.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("shift+enter", "ctrl+m"), key.WithHelp("enter", "insert newline"))
	t.Prompt = ""
	t.ShowLineNumbers = false

	styles := t.Styles()
	styles.Focused.CursorLine = styles.Focused.CursorLine.UnsetBackground()

	t.MaxWidth = maxWidth - len(t.Prompt) - 2*textareaPadding

	t.Focus()
	t.SetHeight(1)
	t.SetStyles(styles)

	return model{
		agent:     agent,
		spinner:   s,
		statusbar: statusbar.New(agent.Chat.Model, modelContextWindow(agent.ModelInfo)),
		help:      help.New(),
		textarea:  t,
		keys:      defaultKeyMap(),
	}
}

// Init implements the tea.Model interface. It is the first function that will be
// called. It returns an optional initial command. To not perform an initial command
// return nil.
func (m model) Init() tea.Cmd {
	return nil
}

// Update implements the tea.Model interface. It is called when a message is received.
// Use it to inspect messages and, in response, update the model and/or send a command.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.textarea.SetWidth(msg.Width)
		m.help.SetWidth(msg.Width)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Send):
			var cmd tea.Cmd
			m, cmd = m.handleSend()
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.Cancel):
			var cmd tea.Cmd
			m, cmd = m.handleCancel()
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}
	case runFinishedMsg:
		m.isWorking = false
		m.cancel = nil
	}

	if m.isWorking {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	{
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View implements the tea.Model interface. It renders the program's UI, which can be a
// string or a Layer. The view is rendered after every Update.
func (m model) View() tea.View {
	var parts []string

	if m.isWorking {
		parts = append(parts, m.spinner.View()+" Working...")
	}

	m.statusbar.SetStats(len(m.agent.Chat.Messages), toStatusbarUsage(m.agent.Usage()))
	parts = append(parts, textareaStyle.Render(m.textarea.View()))
	parts = append(parts, m.statusbar.View())
	parts = append(parts, m.help.View(m.keys))

	return tea.NewView(strings.Join(parts, "\n"))
}

// handleSend handles the send message action.
func (m model) handleSend() (model, tea.Cmd) {
	if m.isWorking {
		// Do nothing while a request is in flight.
		return m, nil
	}

	trimmed := strings.TrimSpace(m.textarea.Value())
	m.textarea.SetValue("")
	if trimmed == "" {
		// Do nothing with an empty message.
		return m, nil
	}

	// Start a new inference run.
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.isWorking = true
	events := m.agent.Run(ctx, thingmodel.NewUserMessage(trimmed))

	// Process events.
	p := m.p
	go func() {
		for event := range events {
			m.renderToScrollback(m.renderEvent(event))
		}
		p.Send(runFinishedMsg{})
	}()

	return m, m.spinner.Tick
}

// handleCancel handles the cancel action.
func (m model) handleCancel() (model, tea.Cmd) {
	if m.cancel != nil {
		m.cancel()
	}

	m.isWorking = false
	m.cancel = nil

	return m, nil
}

// renderToScrollback renders text to the scrollback buffer.
func (m *model) renderToScrollback(s string) {
	// NOTE: We split the string into lines and send them individually to the scrollback
	// buffer to avoid corrupting the terminal output when a single string exceeds the
	// terminal's height.
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		m.p.Println(scanner.Text())
	}
}

// renderEvent renders an event to a string.
func (m *model) renderEvent(event agent.Event) string {
	switch event.Kind {
	case agent.KindAssistant, agent.KindFinal, agent.KindUser:
		return m.renderMessageEvent(event)
	case agent.KindToolCall:
		return toolStyle.Render(m.renderToolCallEvent(event))
	case agent.KindError:
		return errorStyle.Render("error: " + event.Message)
	default:
		return ""
	}
}

// renderMessageEvent renders a message event to a string.
func (m *model) renderMessageEvent(event agent.Event) string {
	switch event.Kind {
	case agent.KindAssistant, agent.KindFinal:
		return assistantMessageStyle.Render(m.mustRenderMarkdown(event.Message))
	case agent.KindUser:
		return userMessageStyle.Render(m.mustRenderMarkdown(event.Message))
	default:
		return ""
	}
}

// renderToolCallEvent renders a tool call event to a string.
func (m *model) renderToolCallEvent(event agent.Event) string {
	var args struct {
		Path       string `json:"path"`
		Command    string `json:"command"`
		Timeout    int    `json:"timeout"`
		OldText    string `json:"oldText"`
		NewText    string `json:"newText"`
		ReplaceAll bool   `json:"replaceAll"`
	}
	_ = json.Unmarshal([]byte(event.ToolInput), &args)

	if event.Tool == "bash" {
		name := event.Tool
		if args.Timeout > 0 {
			name = fmt.Sprintf("%s (timeout: %ds)", name, args.Timeout)
		}
		message := fmt.Sprintf("%s\n```bash\n%s\n```", name, args.Command)
		return toolMessageStyle.Render(m.mustRenderToolCallMarkdown(message))
	}

	if event.Tool == "edit" {
		message := fmt.Sprintf("edit %s\n```diff\n%s\n```", args.Path, renderEditDiff(args.OldText, args.NewText))
		if args.ReplaceAll {
			message = fmt.Sprintf("edit %s (all occurrences)\n```diff\n%s\n```", args.Path, renderEditDiff(args.OldText, args.NewText))
		}
		return toolMessageStyle.Render(m.mustRenderToolCallMarkdown(message))
	}

	input := args.Path
	if input == "" {
		input = string(event.ToolInput)
	}
	return toolMessageStyle.Render(m.mustRenderToolCallMarkdown(fmt.Sprintf("%s %s", event.Tool, input)))
}

// renderEditDiff renders the text replacement as a compact diff.
func renderEditDiff(oldText, newText string) string {
	var b strings.Builder
	for _, line := range strings.Split(oldText, "\n") {
		fmt.Fprintf(&b, "-%s\n", line)
	}
	for _, line := range strings.Split(newText, "\n") {
		fmt.Fprintf(&b, "+%s\n", line)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// mustRenderMarkdown renders a string as markdown using the default style. It panics
// if there is a render error.
func (m *model) mustRenderMarkdown(s string) string {
	return m.mustRenderMarkdownWithStyle(s, styles.DarkStyleConfig)
}

// mustRenderToolCallMarkdown renders a tool call as markdown without colors. It panics
// if there is a render error.
func (m *model) mustRenderToolCallMarkdown(s string) string {
	return m.mustRenderMarkdownWithStyle(s, styles.NoTTYStyleConfig)
}

// mustRenderMarkdownWithStyle renders a string as markdown with the given style. It
// panics if there is a render error.
func (m *model) mustRenderMarkdownWithStyle(s string, style ansi.StyleConfig) string {
	t, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(maxWidth),
	)
	if err != nil {
		panic(err)
	}

	ret, err := t.Render(s)
	if err != nil {
		panic(err)
	}

	// Trimming space and re-adding the leading two spaces makes it easier to style the
	// message with lipgloss.
	return "  " + strings.TrimSpace(ret)
}

// modelContextWindow returns the context window size from model metadata.
func modelContextWindow(info *catwalk.Model) int64 {
	if info == nil {
		return 0
	}
	return info.ContextWindow
}

// toStatusbarUsage adapts agent usage data to the statusbar component.
func toStatusbarUsage(u agent.Usage) statusbar.Usage {
	return statusbar.Usage{
		PromptTokens:      u.PromptTokens,
		CompletionTokens:  u.CompletionTokens,
		CachedTokens:      u.CachedTokens,
		CachedTokensRatio: u.CachedTokensRatio,
	}
}
