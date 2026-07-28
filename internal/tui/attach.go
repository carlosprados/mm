package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/carlosprados/mm/internal/client"
)

// expandAttachments turns a prompt line into validated file paths. It accepts
// several paths on one line, honours quoted or backslash-escaped spaces, expands
// a leading "~" and expands shell globs. Every resulting path is validated here
// so bad input is reported while the prompt is still open, not at send time.
func expandAttachments(line string) ([]string, error) {
	var out []string
	for _, token := range splitPaths(line) {
		path, err := expandTilde(token)
		if err != nil {
			return nil, err
		}

		matches := []string{path}
		if hasGlobMeta(path) {
			m, err := filepath.Glob(path)
			if err != nil {
				return nil, fmt.Errorf("bad pattern %s: %w", token, err)
			}
			if len(m) == 0 {
				return nil, fmt.Errorf("no files match %s", token)
			}
			sort.Strings(m)
			matches = m
		}

		for _, match := range matches {
			if err := client.ValidateAttachment(match); err != nil {
				return nil, err
			}
			out = append(out, match)
		}
	}
	return out, nil
}

// splitPaths splits a line into path tokens on unquoted whitespace. Double
// quotes and backslash escapes let the user type paths containing spaces, which
// are common enough ("My Documents", cloud-synced folders) to be worth supporting.
func splitPaths(line string) []string {
	var (
		tokens []string
		cur    strings.Builder
		quoted bool
		escape bool
	)
	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}

	for _, r := range line {
		switch {
		case escape:
			cur.WriteRune(r)
			escape = false
		case r == '\\':
			escape = true
		case r == '"':
			quoted = !quoted
		case (r == ' ' || r == '\t') && !quoted:
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return tokens
}

// expandTilde resolves a leading "~" or "~/" against the home directory. Only
// the current user's home is supported ("~other" is left untouched).
func expandTilde(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not resolve ~: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

func hasGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

// attachSummary describes the queued attachments for the footer: a count plus
// base names, kept short so the single-line footer doesn't overflow.
func attachSummary(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	names := make([]string, 0, len(paths))
	for _, p := range paths {
		names = append(names, filepath.Base(p))
	}
	label := fmt.Sprintf("%d file", len(paths))
	if len(paths) > 1 {
		label += "s"
	}
	return "[" + label + ": " + truncateDisplay(strings.Join(names, ", "), 40) + "]"
}
