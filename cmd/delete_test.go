package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/carlosprados/mm/internal/client"
)

func TestDeletePreview(t *testing.T) {
	at := time.Date(2026, 7, 28, 9, 15, 0, 0, time.Local)
	base := client.Message{CreateAt: at.UnixMilli(), Author: "@jane.doe"}

	m := base
	m.Text = "deploy listo"
	if got, want := deletePreview(m), "[2026-07-28 09:15] @jane.doe: deploy listo"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// A message with only attachments has no text to show.
	m = base
	if got := deletePreview(m); !strings.Contains(got, "(no text)") {
		t.Errorf("empty body should be marked, got %q", got)
	}

	// Newlines would break the one-line preview.
	m = base
	m.Text = "line one\nline two"
	if got := deletePreview(m); strings.Contains(got, "\n") {
		t.Errorf("preview must stay on one line, got %q", got)
	}

	// Long text is cut by runes, never mid-character.
	m = base
	m.Text = strings.Repeat("á", 200)
	got := deletePreview(m)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("long text should be elided, got %q", got)
	}
	if strings.ContainsRune(got, '�') {
		t.Error("truncation split a multi-byte character")
	}
}
