package tui

import (
	"regexp"
	"sort"
	"strings"

	"github.com/kyokomi/emoji/v2"
)

const maxEmojiResults = 7 // popup rows

// emojiEntry is one searchable emoji: its short name (no colons) and glyph.
type emojiEntry struct {
	short string
	glyph string
}

// emojiList is the full set, built once and sorted for deterministic results.
var emojiList = buildEmojiList()

func buildEmojiList() []emojiEntry {
	cm := emoji.CodeMap() // ":smile:" -> "😄"
	list := make([]emojiEntry, 0, len(cm))
	for code, glyph := range cm {
		list = append(list, emojiEntry{short: strings.Trim(code, ":"), glyph: glyph})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].short < list[j].short })
	return list
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
