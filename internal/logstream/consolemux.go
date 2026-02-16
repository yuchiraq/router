package logstream

import (
	"bytes"
	"io"
)

const (
	ansiReset  = "\033[0m"
	ansiBlue   = "\033[34m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
	ansiCyan   = "\033[36m"
)

// ConsoleMux writes plain logs to broadcast output and colorized logs to console output.
type ConsoleMux struct {
	console   io.Writer
	broadcast io.Writer
}

// NewConsoleMux builds a writer that fans out logs to console and broadcaster.
func NewConsoleMux(console io.Writer, broadcast io.Writer) *ConsoleMux {
	return &ConsoleMux{console: console, broadcast: broadcast}
}

// Write sends original message to broadcast and colorized message to console.
func (m *ConsoleMux) Write(p []byte) (int, error) {
	if m.broadcast != nil {
		if _, err := m.broadcast.Write(p); err != nil {
			return 0, err
		}
	}

	if m.console != nil {
		decorated := colorize(p)
		if _, err := m.console.Write(decorated); err != nil {
			return 0, err
		}
	}

	return len(p), nil
}

func colorize(p []byte) []byte {
	levelColor := ""
	switch {
	case bytes.Contains(p, []byte("[ERROR]")), bytes.Contains(p, []byte("[FATAL]")):
		levelColor = ansiRed
	case bytes.Contains(p, []byte("[WARN]")):
		levelColor = ansiYellow
	case bytes.Contains(p, []byte("[DEBUG]")):
		levelColor = ansiCyan
	case bytes.Contains(p, []byte("[INFO]")):
		levelColor = ansiBlue
	default:
		return p
	}

	out := make([]byte, 0, len(p)+len(levelColor)+len(ansiReset))
	out = append(out, []byte(levelColor)...)
	out = append(out, p...)
	out = append(out, []byte(ansiReset)...)
	return out
}
