package ui

import (
	"context"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/WillHouMoe/pandoc-tui/internal/convert"
	"github.com/WillHouMoe/pandoc-tui/internal/pandoc"
)

type runState int

const (
	runRunning runState = iota
	runSucceeded
	runFailed
	runCancelled
)

// runEvent is one thing that happened while pandoc was running.
type runEvent struct {
	line   string
	stream pandoc.Stream
	done   bool
	err    error
}

type runEventMsg struct{ event runEvent }

// closeRunMsg asks the shell to leave the run screen.
type closeRunMsg struct{}

// runner drives one pandoc invocation and shows what it printed.
type runner struct {
	conv    convert.Converter
	req     convert.Request
	args    []string
	command string

	events     <-chan runEvent
	cancel     context.CancelFunc
	cancelling bool

	lines []string
	state runState
	err   error
	spin  spinner.Model

	output     string
	outputSize int64
	hasOutput  bool
}

// startRunner launches pandoc in the background and returns a command that
// waits for its first line of output.
func startRunner(client *pandoc.Client, conv convert.Converter, req convert.Request) (runner, tea.Cmd) {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(colAccent)

	r := runner{
		conv:   conv,
		req:    req,
		spin:   spin,
		state:  runRunning,
		output: req.Output,
	}

	args, err := conv.Args(req)
	if err != nil {
		r.state = runFailed
		r.err = err
		return r, nil
	}
	r.args = args
	r.command = client.CommandLine(args)

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	events := make(chan runEvent, 128)
	r.events = events
	go func() {
		defer close(events)
		runErr := client.Run(ctx, args, func(s pandoc.Stream, line string) {
			select {
			case events <- runEvent{line: line, stream: s}:
			case <-ctx.Done():
			}
		})
		select {
		case events <- runEvent{done: true, err: runErr}:
		case <-ctx.Done():
		}
	}()

	return r, tea.Batch(spin.Tick, waitForRun(events))
}

func waitForRun(events <-chan runEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return nil
		}
		return runEventMsg{event: event}
	}
}

func (r runner) Update(msg tea.Msg) (runner, tea.Cmd) {
	switch msg := msg.(type) {
	case runEventMsg:
		if !msg.event.done {
			r.lines = append(r.lines, msg.event.line)
			return r, waitForRun(r.events)
		}
		if r.cancel != nil {
			defer r.cancel()
		}
		switch {
		case r.cancelling:
			r.state = runCancelled
		case msg.event.err != nil:
			r.state = runFailed
			r.err = msg.event.err
		default:
			r.state = runSucceeded
			if info, err := os.Stat(r.output); err == nil {
				r.outputSize = info.Size()
				r.hasOutput = true
			}
		}
		if r.cancelling {
			return r, func() tea.Msg { return closeRunMsg{} }
		}
		return r, nil

	case spinner.TickMsg:
		if r.state != runRunning {
			return r, nil
		}
		var cmd tea.Cmd
		r.spin, cmd = r.spin.Update(msg)
		return r, cmd

	case tea.KeyMsg:
		switch {
		case keyMatches(msg, keys.Cancel), msg.String() == "q":
			if r.state == runRunning {
				r.cancelling = true
				if r.cancel != nil {
					r.cancel()
				}
				return r, nil
			}
			return r, func() tea.Msg { return closeRunMsg{} }
		case keyMatches(msg, keys.Confirm):
			if r.state == runRunning {
				return r, nil
			}
			return r, func() tea.Msg { return closeRunMsg{} }
		case keyMatches(msg, keys.Open):
			if r.hasOutput {
				path := r.output
				return r, func() tea.Msg {
					return appOpenedMsg{what: "the converted file", err: openInApp(path)}
				}
			}
		case keyMatches(msg, keys.Reveal):
			if r.hasOutput {
				return r, revealCmd(r.output)
			}
		case keyMatches(msg, keys.Run):
			if r.state != runRunning {
				return r, func() tea.Msg { return retryRunMsg{} }
			}
		}
	}
	return r, nil
}

func (r runner) render(width, height int) frame {
	var b blockBuilder

	status := []string{}
	switch r.state {
	case runRunning:
		status = append(status, r.spin.View()+" "+stValue.Render("pandoc is working…"))
	case runSucceeded:
		line := stOK.Render("✓ wrote ") + stValue.Render(truncate(shortenPath(r.output), width-24))
		if r.hasOutput {
			line += stFaint.Render("  " + humanSize(r.outputSize))
		}
		status = append(status, line)
		if len(r.lines) == 0 {
			status = append(status, stFaint.Render("pandoc said nothing, which is what a clean run looks like."))
		}
	case runFailed:
		status = append(status, stErr.Render("✗ conversion failed"))
		if r.err != nil {
			for _, line := range wrap(r.err.Error(), width-4) {
				status = append(status, stErr.Render(line))
			}
		}
	case runCancelled:
		status = append(status, stWarn.Render("conversion cancelled"))
	}
	b.panel(width, "Status", status)
	b.blank()

	// The full command also lands in the log below when it is long, so the
	// panel only has to show enough of it to be recognisable.
	const maxCommandLines = 4
	command := wrap(r.command, width-4)
	if len(command) > maxCommandLines {
		command = append(command[:maxCommandLines], stFaint.Render("…"))
	}
	b.panel(width, "Command", command)
	b.blank()

	// The log panel takes whatever room is left so the output is readable on
	// any window size.
	header := len(b.lines)
	room := height - header - 3
	if room < 1 {
		room = 1
	}

	log := r.lines
	if len(log) > room {
		log = log[len(log)-room:]
	}
	logBody := make([]string, 0, room)
	logBody = append(logBody, log...)
	if len(logBody) == 0 {
		placeholder := "pandoc produced no messages"
		if r.state == runRunning {
			placeholder = "waiting for pandoc…"
		}
		logBody = append(logBody, stFaint.Render(placeholder))
	}
	b.panel(width, "Output", logBody)

	footer := []keyHint{{keys: "esc", desc: "cancel"}}
	switch r.state {
	case runSucceeded:
		footer = []keyHint{
			{keys: "o", desc: "open file"},
			{keys: "f", desc: "reveal"},
			{keys: "ctrl+r", desc: "run again"},
			{keys: "esc", desc: "back"},
		}
	case runFailed, runCancelled:
		footer = []keyHint{
			{keys: "ctrl+r", desc: "run again"},
			{keys: "esc", desc: "back"},
		}
	}

	return frame{lines: b.lines, cursorLine: -1, footer: footer}
}
