package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

// upload records what the fake server received for one /files request.
type upload struct {
	channelID string
	filename  string
	content   string
}

// fakeServer stands in for Mattermost: it accepts file uploads and post
// creation, recording both so the tests can assert on the wire behaviour.
type fakeServer struct {
	uploads []upload
	posts   []*model.Post
	nextID  int
}

func newFakeMM(t *testing.T) (*MM, *fakeServer) {
	t.Helper()
	fs := &fakeServer{}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/files", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("could not parse multipart body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		headers := r.MultipartForm.File["files"]
		if len(headers) != 1 {
			t.Errorf("expected 1 file part, got %d", len(headers))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f, err := headers[0].Open()
		if err != nil {
			t.Fatalf("could not open uploaded part: %v", err)
		}
		defer f.Close()
		content, err := io.ReadAll(f)
		if err != nil {
			t.Fatalf("could not read uploaded part: %v", err)
		}

		fs.nextID++
		fs.uploads = append(fs.uploads, upload{
			channelID: r.FormValue("channel_id"),
			filename:  headers[0].Filename,
			content:   string(content),
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&model.FileUploadResponse{
			FileInfos: []*model.FileInfo{{Id: fileIDFor(fs.nextID)}},
		})
	})
	mux.HandleFunc("/api/v4/posts", func(w http.ResponseWriter, r *http.Request) {
		var post model.Post
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			t.Errorf("could not decode post: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		post.Id = "post1"
		fs.posts = append(fs.posts, &post)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&post)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &MM{Client: model.NewAPIv4Client(srv.URL), UserID: "me"}, fs
}

func fileIDFor(n int) string {
	return fmt.Sprintf("file%d", n)
}

// writeTempFile creates a file with the given content and returns its path.
func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("could not write %s: %v", path, err)
	}
	return path
}

func TestUploadFilesReturnsIDsInOrder(t *testing.T) {
	mm, fs := newFakeMM(t)
	first := writeTempFile(t, "report.pdf", "pdf-bytes")
	second := writeTempFile(t, "shot.png", "png-bytes")

	ids, err := mm.UploadFiles(context.Background(), "chan1", []string{first, second})
	if err != nil {
		t.Fatalf("UploadFiles: %v", err)
	}

	if got, want := len(ids), 2; got != want {
		t.Fatalf("got %d ids, want %d", got, want)
	}
	if ids[0] == ids[1] {
		t.Errorf("expected distinct file ids, got %v", ids)
	}
	if len(fs.uploads) != 2 {
		t.Fatalf("got %d uploads, want 2", len(fs.uploads))
	}
	// Order matters: Mattermost renders attachments in FileIds order.
	if fs.uploads[0].filename != "report.pdf" || fs.uploads[1].filename != "shot.png" {
		t.Errorf("uploads out of order: %+v", fs.uploads)
	}
	if fs.uploads[0].content != "pdf-bytes" {
		t.Errorf("got content %q, want %q", fs.uploads[0].content, "pdf-bytes")
	}
	if fs.uploads[0].channelID != "chan1" {
		t.Errorf("got channel_id %q, want chan1", fs.uploads[0].channelID)
	}
}

func TestUploadFilesSendsOnlyBasename(t *testing.T) {
	mm, fs := newFakeMM(t)
	path := writeTempFile(t, "notes.txt", "hello")

	if _, err := mm.UploadFiles(context.Background(), "chan1", []string{path}); err != nil {
		t.Fatalf("UploadFiles: %v", err)
	}
	if fs.uploads[0].filename != "notes.txt" {
		t.Errorf("got filename %q, want notes.txt (no local directories leaked)", fs.uploads[0].filename)
	}
}

func TestUploadFilesRejectsBadInput(t *testing.T) {
	ok := writeTempFile(t, "ok.txt", "content")
	empty := writeTempFile(t, "empty.txt", "")

	tests := []struct {
		name      string
		channelID string
		paths     []string
		wantErr   string
	}{
		{"no channel", "", []string{ok}, "channel is required"},
		{"missing file", "chan1", []string{filepath.Join(t.TempDir(), "nope.txt")}, "could not read"},
		{"directory", "chan1", []string{t.TempDir()}, "is a directory"},
		{"empty file", "chan1", []string{empty}, "is empty"},
		{"too many files", "chan1", make([]string, MaxFilesPerPost+1), "too many files"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mm, fs := newFakeMM(t)
			_, err := mm.UploadFiles(context.Background(), tc.channelID, tc.paths)
			if err == nil {
				t.Fatalf("expected an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("got error %q, want it to contain %q", err, tc.wantErr)
			}
			if len(fs.uploads) != 0 {
				t.Errorf("nothing should have been uploaded, got %d", len(fs.uploads))
			}
		})
	}
}

func TestSendFilesToChannelIDAttachesFileIDs(t *testing.T) {
	mm, fs := newFakeMM(t)
	path := writeTempFile(t, "log.txt", "stacktrace")

	postID, err := mm.SendFilesToChannelID(context.Background(), "chan1", "mira esto", []string{path})
	if err != nil {
		t.Fatalf("SendFilesToChannelID: %v", err)
	}
	if postID != "post1" {
		t.Errorf("got post id %q, want post1", postID)
	}
	if len(fs.posts) != 1 {
		t.Fatalf("got %d posts, want 1", len(fs.posts))
	}
	post := fs.posts[0]
	if post.Message != "mira esto" {
		t.Errorf("got message %q, want %q", post.Message, "mira esto")
	}
	if len(post.FileIds) != 1 {
		t.Fatalf("got %d file ids on the post, want 1", len(post.FileIds))
	}
	if post.FileIds[0] != fileIDFor(1) {
		t.Errorf("post claimed %q, want the uploaded %q", post.FileIds[0], fileIDFor(1))
	}
}

// An attachment-only message is valid; a message with neither body nor files is not.
func TestSendFilesToChannelIDBodyOptionalWithAttachment(t *testing.T) {
	mm, fs := newFakeMM(t)
	path := writeTempFile(t, "photo.png", "png")

	if _, err := mm.SendFilesToChannelID(context.Background(), "chan1", "", []string{path}); err != nil {
		t.Fatalf("attachment-only send should succeed: %v", err)
	}
	if len(fs.posts) != 1 || len(fs.posts[0].FileIds) != 1 {
		t.Fatalf("expected one post with one attachment, got %+v", fs.posts)
	}

	if _, err := mm.SendFilesToChannelID(context.Background(), "chan1", "", nil); err == nil {
		t.Error("expected an error when neither message nor files are given")
	}
}

// A post is only created once every upload succeeded, so a bad path leaves no
// half-sent message in the channel.
func TestSendFilesToChannelIDNoPostWhenUploadFails(t *testing.T) {
	mm, fs := newFakeMM(t)
	missing := filepath.Join(t.TempDir(), "gone.txt")

	if _, err := mm.SendFilesToChannelID(context.Background(), "chan1", "hola", []string{missing}); err == nil {
		t.Fatal("expected an error for a missing file")
	}
	if len(fs.posts) != 0 {
		t.Errorf("no post should have been created, got %d", len(fs.posts))
	}
}
