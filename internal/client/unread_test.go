package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

// unreadFake serves a team with four channels: #general (read), #dev (unread,
// no mention), a DM with "peer" (unread, mentioned, most recent activity is
// older than #dev's) and #quiet, whose LastPostAt moved without any new
// message (an edit or a system post). Viewed-at is 100 everywhere.
func newUnreadFake(t *testing.T, postFetches *atomic.Int32) *MM {
	t.Helper()
	const viewed = 100
	chans := []*model.Channel{
		{Id: "general", Name: "general", Type: model.ChannelTypeOpen, LastPostAt: 90, TotalMsgCount: 5},
		{Id: "dev", Name: "dev", Type: model.ChannelTypeOpen, LastPostAt: 300, TotalMsgCount: 10},
		{Id: "me__peer", Name: "me__peer", Type: model.ChannelTypeDirect, LastPostAt: 200, TotalMsgCount: 4},
		{Id: "quiet", Name: "quiet", Type: model.ChannelTypeOpen, LastPostAt: 400, TotalMsgCount: 2},
	}
	members := []model.ChannelMember{
		{ChannelId: "general", LastViewedAt: viewed, MsgCount: 5},
		{ChannelId: "dev", LastViewedAt: viewed, MsgCount: 7},
		{ChannelId: "me__peer", LastViewedAt: viewed, MsgCount: 3, MentionCount: 1},
		{ChannelId: "quiet", LastViewedAt: viewed, MsgCount: 2},
	}
	// Newest first, like the API. Posts at or before viewed are already seen.
	posts := map[string][]*model.Post{
		"dev": {
			{Id: "d4", UserId: "ann", CreateAt: 300, Message: "deploy done"},
			{Id: "d3", UserId: "bob", CreateAt: 250, Message: "", Type: model.PostTypeJoinChannel},
			{Id: "d2", UserId: "bob", CreateAt: 150, Message: "deploying"},
			{Id: "d1", UserId: "ann", CreateAt: 50, Message: "old news"},
		},
		"me__peer": {
			{Id: "p1", UserId: "peer", CreateAt: 200, Message: "can you review?"},
		},
		"quiet": {
			{Id: "q1", UserId: "bob", CreateAt: 400, Type: model.PostTypeJoinChannel},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/users/me/teams/team/channels", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(chans)
	})
	mux.HandleFunc("/api/v4/users/me/teams/team/channels/members", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(members)
	})
	mux.HandleFunc("/api/v4/channels/", func(w http.ResponseWriter, r *http.Request) {
		postFetches.Add(1)
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v4/channels/"), "/posts")
		list := &model.PostList{Posts: map[string]*model.Post{}}
		for _, p := range posts[id] {
			list.Order = append(list.Order, p.Id)
			list.Posts[p.Id] = p
		}
		json.NewEncoder(w).Encode(list)
	})
	mux.HandleFunc("/api/v4/users/ids", func(w http.ResponseWriter, r *http.Request) {
		var ids []string
		json.NewDecoder(r.Body).Decode(&ids)
		users := make([]*model.User, 0, len(ids))
		for _, id := range ids {
			users = append(users, &model.User{Id: id, Username: id})
		}
		json.NewEncoder(w).Encode(users)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &MM{Client: model.NewAPIv4Client(srv.URL), UserID: "me", TeamID: "team"}
}

func TestUnread(t *testing.T) {
	var fetches atomic.Int32
	mm := newUnreadFake(t, &fetches)

	got, err := mm.Unread(context.Background(), UnreadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d unread channels, want 2 (#general is read, #quiet has nothing new): %+v", len(got), got)
	}

	// The mentioned DM wins over #dev even though #dev is more recent.
	dm, dev := got[0], got[1]
	if dm.Name != "@peer" || dm.Mentions != 1 || dm.Count != 1 {
		t.Errorf("first = %+v, want the DM as @peer with 1 mention and 1 unread", dm)
	}
	if dev.Name != "dev" || dev.Count != 3 {
		t.Errorf("second = %q with %d unread, want dev with 3 (server count)", dev.Name, dev.Count)
	}

	// Only posts after LastViewedAt, system posts skipped, oldest first.
	var texts []string
	for _, m := range dev.Messages {
		texts = append(texts, m.Text)
	}
	if strings.Join(texts, "|") != "deploying|deploy done" {
		t.Errorf("dev messages = %q, want [deploying deploy done]", texts)
	}
	if !dev.Truncated() {
		t.Error("dev: server says 3 unread but 2 were shown; Truncated() should be true")
	}
}

func TestUnreadFilters(t *testing.T) {
	var fetches atomic.Int32
	mm := newUnreadFake(t, &fetches)

	got, err := mm.Unread(context.Background(), UnreadOptions{MentionsOnly: true, CountsOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "@peer" {
		t.Fatalf("mentions only = %+v, want just the DM", got)
	}
	if len(got[0].Messages) != 0 || fetches.Load() != 0 {
		t.Errorf("counts only fetched posts (%d requests, %d messages)", fetches.Load(), len(got[0].Messages))
	}
	if got[0].Count != 1 {
		t.Errorf("count = %d, want 1 from the server counters", got[0].Count)
	}
}
