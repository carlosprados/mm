package tui

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

const maxEmojiResults = 7 // popup rows

// emojiEntry is one searchable emoji: its short name (no colons) and glyph.
type emojiEntry struct {
	short string
	glyph string
}

// We source emoji from model.SystemEmojis (the exact name set Mattermost
// accepts) rather than a third-party set, so reaction names always validate
// server-side and shortcodes match what Mattermost renders.
var (
	emojiList   = buildEmojiList()
	emojiByName = buildEmojiByName()
)

// codepointToGlyph turns "1f604" or "1f468-200d-1f4bb" into the glyph string.
func codepointToGlyph(cp string) string {
	var b strings.Builder
	for _, part := range strings.Split(cp, "-") {
		n, err := strconv.ParseInt(part, 16, 32)
		if err != nil {
			return ""
		}
		b.WriteRune(rune(n))
	}
	return b.String()
}

func buildEmojiList() []emojiEntry {
	list := make([]emojiEntry, 0, len(model.SystemEmojis))
	for name, cp := range model.SystemEmojis {
		list = append(list, emojiEntry{short: name, glyph: codepointToGlyph(cp)})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].short < list[j].short })
	return list
}

func buildEmojiByName() map[string]string {
	m := make(map[string]string, len(model.SystemEmojis))
	for name, cp := range model.SystemEmojis {
		m[name] = codepointToGlyph(cp)
	}
	return m
}

// emojiGlyph returns the glyph for a short name, or :name: if unknown (custom).
func emojiGlyph(name string) string {
	if g, ok := emojiByName[name]; ok && g != "" {
		return g
	}
	return ":" + name + ":"
}

// emojiQueryRe matches a trailing ":word" token (the emoji being typed), at
// least 2 chars. The colon must start the value or follow whitespace, so
// "10:30" doesn't trigger.
var emojiQueryRe = regexp.MustCompile(`(?:^|\s):([a-zA-Z0-9_+\-]{2,})$`)

// activeEmojiQuery returns the lowercase emoji query at the end of val, or "".
func activeEmojiQuery(val string) string {
	m := emojiQueryRe.FindStringSubmatch(val)
	if m == nil {
		return ""
	}
	return strings.ToLower(m[1])
}

// searchEmoji returns matches for query, prefix matches first, capped.
func searchEmoji(query string) []emojiEntry {
	var prefix, sub []emojiEntry
	for _, e := range emojiList {
		switch idx := strings.Index(e.short, query); {
		case idx == 0:
			prefix = append(prefix, e)
		case idx > 0:
			sub = append(sub, e)
		}
	}
	res := append(prefix, sub...)
	if len(res) > maxEmojiResults {
		res = res[:maxEmojiResults]
	}
	return res
}
