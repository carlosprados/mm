package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

// deleteFake serves a single post plus its delete, recording what was called.
func newDeleteFake(t *testing.T, p *model.Post, status int) (*MM, *[]string) {
	t.Helper()
	var calls []string

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/posts/"+p.Id, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method)
		if r.Method == http.MethodDelete {
			if status != http.StatusOK {
				w.WriteHeader(status)
				json.NewEncoder(w).Encode(&model.AppError{Message: "no permission"})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "OK"})
			return
		}
		json.NewEncoder(w).Encode(p)
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
	return &MM{Client: model.NewAPIv4Client(srv.URL), UserID: "me"}, &calls
}

func TestGetMessage(t *testing.T) {
	id := strings.Repeat("a", 26)
	mm, _ := newDeleteFake(t, &model.Post{
		Id: id, UserId: "me", Message: "hola", CreateAt: 1000,
		FileIds: model.StringArray{"f1"},
	}, http.StatusOK)

	msg, err := mm.GetMessage(context.Background(), id)
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	if msg.ID != id || msg.Text != "hola" || msg.Author != "@user-me" {
		t.Errorf("unexpected message: %+v", msg)
	}
	if !msg.Own {
		t.Error("post authored by the current user should be Own")
	}
	if len(msg.FileIDs) != 1 {
		t.Errorf("attachments lost: %v", msg.FileIDs)
	}
}

func TestGetMessageNotOwn(t *testing.T) {
	id := strings.Repeat("b", 26)
	mm, _ := newDeleteFake(t, &model.Post{Id: id, UserId: "someone", Message: "x"}, http.StatusOK)

	msg, err := mm.GetMessage(context.Background(), id)
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	if msg.Own {
		t.Error("someone else's post must not be flagged Own")
	}
}

func TestDeletePost(t *testing.T) {
	id := strings.Repeat("c", 26)
	mm, calls := newDeleteFake(t, &model.Post{Id: id, UserId: "me"}, http.StatusOK)

	if err := mm.DeletePost(context.Background(), id); err != nil {
		t.Fatalf("DeletePost: %v", err)
	}
	if len(*calls) != 1 || (*calls)[0] != http.MethodDelete {
		t.Errorf("expected exactly one DELETE, got %v", *calls)
	}
}

func TestDeletePostServerRefusal(t *testing.T) {
	id := strings.Repeat("d", 26)
	mm, _ := newDeleteFake(t, &model.Post{Id: id, UserId: "someone"}, http.StatusForbidden)

	err := mm.DeletePost(context.Background(), id)
	if err == nil {
		t.Fatal("a refused delete must surface an error")
	}
	if !strings.Contains(err.Error(), "could not delete message") {
		t.Errorf("error should be wrapped with context, got %q", err)
	}
}
