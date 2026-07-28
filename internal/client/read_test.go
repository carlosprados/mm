package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestParsePostRef(t *testing.T) {
	id := strings.Repeat("a", 26)
	other := strings.Repeat("b", 26)

	ok := map[string]string{
		id:                                    id,
		"  " + id + "  ":                      id,
		"https://chat.acme.com/acme/pl/" + id: id,
		"https://chat.acme.com/acme/pl/" + id + "/":            id,
		"https://chat.acme.com/acme/pl/" + id + "?highlight=1": id,
		"http://localhost:8065/team/pl/" + other:               other,
	}
	for in, want := range ok {
		got, err := ParsePostRef(in)
		if err != nil {
			t.Errorf("ParsePostRef(%q) errored: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParsePostRef(%q) = %q, want %q", in, got, want)
		}
	}

	bad := []string{
		"",
		"   ",
		"not-an-id",
		strings.Repeat("a", 25), // too short
		strings.Repeat("a", 27), // too long
		"https://chat.acme.com/acme/channels/town-square", // a channel link, not a permalink
	}
	for _, in := range bad {
		if got, err := ParsePostRef(in); err == nil {
			t.Errorf("ParsePostRef(%q) = %q, want an error", in, got)
		}
	}
}

// readFake serves a fixed page of posts plus the user lookups ReadMessages does.
func newReadFake(t *testing.T, posts []*model.Post) *MM {
	t.Helper()
	order := make([]string, 0, len(posts))
	byID := map[string]*model.Post{}
	for _, p := range posts { // callers pass newest-first, like the API
		order = append(order, p.Id)
		byID[p.Id] = p
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/channels/chan1/posts", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(&model.PostList{Order: order, Posts: byID})
	})
	mux.HandleFunc("/api/v4/users/ids", func(w http.ResponseWriter, r *http.Request) {
		var ids []string
		json.NewDecoder(r.Body).Decode(&ids)
		users := make([]*model.User, 0, len(ids))
		for _, id := range ids {
			users = append(users, &model.User{Id: id, Username: "user-" + id})
		}
		json.NewEncoder(w).Encode(users)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &MM{Client: model.NewAPIv4Client(srv.URL), UserID: "me"}
}

func post(id, user, text string, createAt int64) *model.Post {
	return &model.Post{Id: id, UserId: user, Message: text, CreateAt: createAt}
}

func TestReadMessagesIsChronologicalAndCarriesIDs(t *testing.T) {
	mm := newReadFake(t, []*model.Post{ // newest first, as the API returns
		post(strings.Repeat("c", 26), "me", "third", 300),
		post(strings.Repeat("b", 26), "other", "second", 200),
		post(strings.Repeat("a", 26), "me", "first", 100),
	})

	msgs, err := mm.ReadMessagesFromChannelID(context.Background(), "chan1", ReadOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ReadMessagesFromChannelID: %v", err)
	}

	if len(msgs) != 3 {
		t.Fatalf("got %d messages, want 3", len(msgs))
	}
	if msgs[0].Text != "first" || msgs[2].Text != "third" {
		t.Errorf("not chronological: %q … %q", msgs[0].Text, msgs[2].Text)
	}
	// The post ID is the whole point: it must survive to the caller.
	if msgs[0].ID != strings.Repeat("a", 26) {
		t.Errorf("post ID lost: %q", msgs[0].ID)
	}
	if !msgs[0].Own || msgs[1].Own {
		t.Errorf("Own flags wrong: %v, %v", msgs[0].Own, msgs[1].Own)
	}
	if msgs[0].Author != "@user-me" {
		t.Errorf("author = %q, want @user-me", msgs[0].Author)
	}
}

func TestReadMessagesOnlyMine(t *testing.T) {
	mm := newReadFake(t, []*model.Post{
		post(strings.Repeat("c", 26), "me", "mine two", 300),
		post(strings.Repeat("b", 26), "other", "theirs", 200),
		post(strings.Repeat("a", 26), "me", "mine one", 100),
	})

	msgs, err := mm.ReadMessagesFromChannelID(context.Background(), "chan1", ReadOptions{OnlyMine: true})
	if err != nil {
		t.Fatalf("ReadMessagesFromChannelID: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	for _, m := range msgs {
		if !m.Own {
			t.Errorf("mine-only returned someone else's message: %q", m.Text)
		}
	}
}

func TestNthOwnPostID(t *testing.T) {
	newest, middle, oldest := strings.Repeat("c", 26), strings.Repeat("b", 26), strings.Repeat("a", 26)
	mm := newReadFake(t, []*model.Post{
		post(newest, "me", "mine three", 300),
		post(strings.Repeat("x", 26), "other", "theirs", 250),
		post(middle, "me", "mine two", 200),
		post(oldest, "me", "mine one", 100),
	})
	ctx := context.Background()

	for n, want := range map[int]string{1: newest, 2: middle, 3: oldest} {
		got, err := mm.NthOwnPostID(ctx, "chan1", n)
		if err != nil {
			t.Fatalf("NthOwnPostID(%d): %v", n, err)
		}
		if got != want {
			t.Errorf("NthOwnPostID(%d) = %q, want %q", n, got, want)
		}
	}

	// LastOwnPostID must stay equivalent to nth=1 (it is a wrapper now).
	last, err := mm.LastOwnPostID(ctx, "chan1")
	if err != nil {
		t.Fatalf("LastOwnPostID: %v", err)
	}
	if last != newest {
		t.Errorf("LastOwnPostID = %q, want %q", last, newest)
	}

	if _, err := mm.NthOwnPostID(ctx, "chan1", 4); err == nil {
		t.Error("asking beyond the available own messages should error")
	} else if !strings.Contains(err.Error(), "only found 3") {
		t.Errorf("error should say how many were found, got %q", err)
	}
	if _, err := mm.NthOwnPostID(ctx, "chan1", 0); err == nil {
		t.Error("nth 0 should error")
	}
}

func TestNthOwnPostIDNoOwnMessages(t *testing.T) {
	mm := newReadFake(t, []*model.Post{post(strings.Repeat("x", 26), "other", "theirs", 100)})
	_, err := mm.NthOwnPostID(context.Background(), "chan1", 1)
	if err == nil || !strings.Contains(err.Error(), "no message of yours") {
		t.Errorf("got %v, want a 'no message of yours' error", err)
	}
}

// Guard the documented default so a caller passing 0 still gets a page.
func TestReadMessagesDefaultLimit(t *testing.T) {
	if DefaultReadLimit != 20 {
		t.Errorf("DefaultReadLimit = %d, want 20 (documented in the CLI flag)", DefaultReadLimit)
	}
	var posts []*model.Post
	for i := 0; i < 3; i++ {
		posts = append(posts, post(fmt.Sprintf("%026d", i), "me", "x", int64(i)))
	}
	mm := newReadFake(t, posts)
	if _, err := mm.ReadMessagesFromChannelID(context.Background(), "chan1", ReadOptions{}); err != nil {
		t.Fatalf("zero limit should be defaulted, got %v", err)
	}
}
