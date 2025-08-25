package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SplitLog holds two viewports and buffers for logs
type SplitLog struct {
	copyAddr      string
	pasteAddr     string
	logChannel    <-chan string
	leftViewport  viewport.Model
	rightViewport viewport.Model
	leftBuffer    strings.Builder
	rightBuffer   strings.Builder
	lastCopy      time.Time
	lastPaste     time.Time
	width         int
	height        int
}

func NewSplitLog(copyAddr, pasteAddr string, width, height int, logChan <-chan string) *SplitLog {
	w := width / 2
	if w < 0 {
		w = 0
	}
	lvp := viewport.New(w, height)
	rvp := viewport.New(width-w, height)
	return &SplitLog{
		copyAddr:      strings.ToLower(copyAddr),
		pasteAddr:     strings.ToLower(pasteAddr),
		logChannel:    logChan,
		leftViewport:  lvp,
		rightViewport: rvp,
		lastCopy:      time.Now(),
		lastPaste:     time.Now(),
		width:         width,
		height:        height,
	}
}

// UpdateLog processes a new line and updates the appropriate buffer/viewport.
func (sl *SplitLog) UpdateLog(line string) tea.Cmd {
	l := strings.ToLower(line)
	switch {
	case isCopyLogLine(l, sl.copyAddr):
		if !sl.isDuplicateLeftLog(line) {
			sl.leftBuffer.WriteString(colorLogLine(line, true) + "\n")
			sl.leftViewport.SetContent(sl.leftBuffer.String())
			sl.leftViewport.GotoBottom()
			sl.lastCopy = time.Now()
		}
	case isPasteLogLine(l, sl.pasteAddr):
		if !sl.isDuplicateRightLog(line) {
			sl.rightBuffer.WriteString(colorLogLine(line, false) + "\n")
			sl.rightViewport.SetContent(sl.rightBuffer.String())
			sl.rightViewport.GotoBottom()
			sl.lastPaste = time.Now()
		}
	default:
		sl.rightBuffer.WriteString(colorLogLine(line, false) + "\n")
		sl.rightViewport.SetContent(sl.rightBuffer.String())
		sl.rightViewport.GotoBottom()
		sl.lastPaste = time.Now()

		sl.rightBuffer.WriteString(colorLogLine(line, false) + "\n")
		sl.rightViewport.SetContent(sl.rightBuffer.String())
		sl.rightViewport.GotoBottom()
		sl.lastPaste = time.Now()
	}
	return nil
}

// Render returns a combined horizontal layout of both log panes.
func (sl *SplitLog) Render() string {
	style := DefaultStyle
	leftView := style.Width(sl.leftViewport.Width).Render(sl.leftViewport.View())
	rightView := style.Width(sl.rightViewport.Width).Render(sl.rightViewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)
}

// Adjust the viewports whenever the parent TUI's window size changes
func (sl *SplitLog) SetSize(width, height int) {
	w := width / 2
	if w < 0 {
		w = 0
	}
	sl.leftViewport.Width = w
	sl.leftViewport.Height = height
	sl.rightViewport.Width = width - w
	sl.rightViewport.Height = height
	sl.width = width
	sl.height = height
}

func (sl *SplitLog) LastCopyUpdate() time.Time  { return sl.lastCopy }
func (sl *SplitLog) LastPasteUpdate() time.Time { return sl.lastPaste }

// isDuplicateLeftLog ensures we skip repeated lines at the end of the buffer
func (sl *SplitLog) isDuplicateLeftLog(line string) bool {
	contents := sl.leftBuffer.String()
	idx := strings.LastIndex(contents, "\n")
	if idx >= 0 && idx < len(contents) && contents[idx+1:] == line {
		return true
	}
	return false
}

// isDuplicateRightLog ensures we skip repeated lines at the end of the buffer
func (sl *SplitLog) isDuplicateRightLog(line string) bool {
	contents := sl.rightBuffer.String()
	idx := strings.LastIndex(contents, "\n")
	if idx >= 0 && idx < len(contents) && contents[idx+1:] == line {
		return true
	}
	return false
}

// isCopyLogLine and isPasteLogLine are trivial checks
func isCopyLogLine(line, address string) bool {
	return strings.Contains(line, "copy") || strings.Contains(line, address)
}

func isPasteLogLine(line, address string) bool {
	return strings.Contains(line, "paste") || strings.Contains(line, address)
}

// colorLogLine applies a different color for copy vs. paste logs
func colorLogLine(line string, isCopy bool) string {
	if isCopy {
		return DefaultStyle.Foreground(lipgloss.Color("39")).Render(line)
	}
	return DefaultStyle.Render(line)
}
