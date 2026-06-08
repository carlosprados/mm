package schedule

import (
	"fmt"
	"strings"
	"time"
)

// ParseTime accepts a relative duration ("+2h", "+90m"), RFC3339, or a local
// datetime ("2006-01-02 15:04" / "2006-01-02T15:04" / "15:04" today). Shared by
// the CLI, the TUI and MCP so all three interpret times identically.
func ParseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("a time is required (e.g. \"2006-01-02 15:04\" or \"+2h\")")
	}
	if strings.HasPrefix(s, "+") {
		d, err := time.ParseDuration(s[1:])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		return time.Now().Add(d), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	if t, err := time.ParseInLocation("15:04", s, time.Local); err == nil {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local), nil
	}
	return time.Time{}, fmt.Errorf("could not parse time %q (try \"2006-01-02 15:04\", RFC3339, or \"+2h\")", s)
}
