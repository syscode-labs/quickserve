package ui

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
)

type Styles struct {
	label   lipgloss.Style
	success lipgloss.Style
	url     lipgloss.Style
	warning lipgloss.Style
	muted   lipgloss.Style
}

func NewStyles(out io.Writer) Styles {
	r := lipgloss.NewRenderer(out)
	if file, ok := out.(*os.File); ok && isatty.IsTerminal(file.Fd()) {
		r.SetColorProfile(termenv.ANSI256)
	}
	return Styles{
		label:   r.NewStyle().Foreground(lipgloss.Color("69")).Bold(true),
		success: r.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),
		url:     r.NewStyle().Foreground(lipgloss.Color("39")).Underline(true),
		warning: r.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
		muted:   r.NewStyle().Foreground(lipgloss.Color("245")),
	}
}

func (s Styles) LabelValue(label, value string) string {
	return fmt.Sprintf("%s %s", s.label.Render(label+":"), value)
}

func (s Styles) URLValue(label, value string) string {
	return fmt.Sprintf("%s %s", s.label.Render(label+":"), s.url.Render(value))
}

func (s Styles) URL(value string) string {
	return s.url.Render(value)
}

func (s Styles) Success(message string) string {
	return s.success.Render(message)
}

func (s Styles) Warning(message string) string {
	return s.warning.Render(message)
}

func (s Styles) Muted(message string) string {
	return s.muted.Render(message)
}
