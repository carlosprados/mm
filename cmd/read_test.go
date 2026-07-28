package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/carlosprados/mm/internal/client"
)

func TestFormatMessageLine(t *testing.T) {
	at := time.Date(2026, 7, 28, 9, 15, 0, 0, time.Local)
	m := client.Message{
		ID:       strings.Repeat("a", 26),
		CreateAt: at.UnixMilli(),
		Author:   "@jane.doe",
		Text:     "deploy listo",
	}

	if got, want := formatMessageLine(m, false), "[09:15] @jane.doe: deploy listo"; got != want {
		t.Errorf("without ID: got %q, want %q", got, want)
	}

	// With --ids the post ID must be present verbatim so it can be copy-pasted
	// into `mm edit --post`.
	got := formatMessageLine(m, true)
	if !strings.Contains(got, m.ID) {
		t.Errorf("with ID: %q does not contain the post ID", got)
	}
	if want := "[09:15] " + m.ID + " @jane.doe: deploy listo"; got != want {
		t.Errorf("with ID: got %q, want %q", got, want)
	}
}
