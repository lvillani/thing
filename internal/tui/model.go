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
	width     int

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

	t.Focus()
	t.SetHeight(1)
	t.SetStyles(styles)

	return model{
		agent:     agent,
		spinner:   s,
		statusbar: statusbar.New(agent.Chat.Model, string(agent.Chat.ReasoningEffort), modelContextWindow(agent.ModelInfo)),
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
		m.width = msg.Width
		m.textarea.SetWidth(msg.Width - textareaStyle.GetHorizontalFrameSize())
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
	messages, errors := m.agent.Run(ctx, thingmodel.NewUserMessage(trimmed))

	// Process messages and terminal errors.
	p := m.p
	go func() {
		for message := range messages {
			m.renderToScrollback(m.renderMessage(message))
		}
		for err := range errors {
			m.renderToScrollback(errorStyle.Render("error: " + err.Error()))
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

// renderMessage renders a conversation message to a string. Tool-result messages
// are intentionally not rendered; the assistant message already shows the requested
// tool call and the model receives the result in the conversation.
func (m *model) renderMessage(message thingmodel.Message) string {
	switch message.Role {
	case thingmodel.MessageRoleUser:
		return userMessageStyle.Render(m.mustRenderMarkdown(message.Content))
	case thingmodel.MessageRoleAssistant:
		var parts []string
		if strings.TrimSpace(message.Content) != "" {
			parts = append(parts, assistantMessageStyle.Render(m.mustRenderMarkdown(message.Content)))
		}
		for _, toolCall := range message.ToolCalls {
			parts = append(parts, toolStyle.Render(m.renderToolCall(toolCall)))
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

// renderToolCall renders a model tool call for the scrollback view.
func (m *model) renderToolCall(toolCall thingmodel.ToolCall) string {
	var args struct {
		Path    string `json:"path"`
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	_ = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)

	if toolCall.Function.Name == "bash" {
		name := toolCall.Function.Name
		if args.Timeout > 0 {
			name = fmt.Sprintf("%s (timeout: %ds)", name, args.Timeout)
		}
		message := fmt.Sprintf("%s\n```bash\n%s\n```", name, args.Command)
		return toolMessageStyle.Render(m.mustRenderToolCallMarkdown(message))
	}

	return toolMessageStyle.Render(m.mustRenderToolCallMarkdown(toolCall.Function.Name + " " + args.Path))
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
	options := []glamour.TermRendererOption{glamour.WithStyles(style)}
	if m.width > 0 {
		options = append(options, glamour.WithWordWrap(m.width))
	}

	t, err := glamour.NewTermRenderer(options...)
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
