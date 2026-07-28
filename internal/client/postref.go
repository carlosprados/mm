package client

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

// ParsePostRef accepts either a bare post ID or a Mattermost permalink and
// returns the post ID. Permalinks are what Mattermost's "Copy link" puts on the
// clipboard (https://host/team/pl/<post_id>), so accepting them saves the user
// from extracting the ID by hand.
func ParsePostRef(ref string) (string, error) {
	s := strings.TrimSpace(ref)
	if s == "" {
		return "", fmt.Errorf("no post reference given")
	}

	if i := strings.LastIndex(s, "/pl/"); i >= 0 {
		s = s[i+len("/pl/"):]
	}
	// Drop any query string or fragment a copied URL may carry.
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	s = strings.Trim(s, "/")

	if !model.IsValidId(s) {
		return "", fmt.Errorf("%q is not a post ID or permalink: expected 26 characters, or a link like https://host/team/pl/<post_id>", ref)
	}
	return s, nil
}
