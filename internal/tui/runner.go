package tui

import (
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type RunOptions struct {
	Input      io.Reader
	Output     io.Writer
	NoRenderer bool
}

func NoRendererFromEnv() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("POMO_TUI_NO_RENDER")), "1")
}

func runProgram(model tea.Model, opts RunOptions) error {
	programOpts := make([]tea.ProgramOption, 0, 3)
	if opts.Input != nil {
		programOpts = append(programOpts, tea.WithInput(opts.Input))
	}
	if opts.Output != nil {
		programOpts = append(programOpts, tea.WithOutput(opts.Output))
	}
	if opts.NoRenderer {
		programOpts = append(programOpts, tea.WithoutRenderer())
	}
	program := tea.NewProgram(model, programOpts...)
	_, err := program.Run()
	return err
}
