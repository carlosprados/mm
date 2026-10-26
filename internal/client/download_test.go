package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

// Mattermost IDs are 26 lowercase alphanumerics; ParsePostRef insists on it.
const (
	testPostID = "p0st00000000000000000000aa"
	testFileID = "f1le00000000000000000000aa"
)

// newDownloadMM serves one post carrying one file, plus that file's bytes.
func newDownloadMM(t *testing.T, name, content string) *MM {
	t.Helper()
	info := &model.FileInfo{Id: testFileID, PostId: testPostID, Name: name, Size: int64(len(content)), MimeType: "application/pdf"}

	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("/api/v4/posts/"+testPostID, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, &model.Post{Id: testPostID, FileIds: []string{testFileID},
			Metadata: &model.PostMetadata{Files: []*model.FileInfo{info}}})
	})
	mux.HandleFunc("/api/v4/files/"+testFileID+"/info", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, info)
	})
	mux.HandleFunc("/api/v4/files/"+testFileID, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		writeJSON(w, map[string]any{"id": "api.not_found", "status_code": 404})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &MM{Client: model.NewAPIv4Client(srv.URL), UserID: "me"}
}

func TestSafeFileName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"report.pdf", "report.pdf"},
		{"../../.bashrc", ".bashrc"},
		{"/etc/passwd", "passwd"},
		{`..\..\evil.exe`, "evil.exe"},
		{"line\nbreak.txt", "linebreak.txt"},
		{"", "file-abc"},
		{"..", "file-abc"},
		{"dir/", "dir"},
	}
	for _, tc := range tests {
		if got := SafeFileName(tc.in, "abc"); got != tc.want {
			t.Errorf("SafeFileName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDownloadFileNeverOverwrites(t *testing.T) {
	mm := newDownloadMM(t, "report.pdf", "new")
	dir := t.TempDir()
	existing := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(existing, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := Attachment{ID: testFileID, Name: "report.pdf"}
	first, err := mm.DownloadFile(context.Background(), a, dir)
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	second, err := mm.DownloadFile(context.Background(), a, dir)
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}

	if want := filepath.Join(dir, "report (1).pdf"); first != want {
		t.Errorf("got %s, want %s", first, want)
	}
	if want := filepath.Join(dir, "report (2).pdf"); second != want {
		t.Errorf("got %s, want %s", second, want)
	}
	if b, _ := os.ReadFile(existing); string(b) != "old" {
		t.Errorf("existing file was overwritten: %q", b)
	}
	if b, _ := os.ReadFile(first); string(b) != "new" {
		t.Errorf("downloaded content = %q, want %q", b, "new")
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".part") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestDownloadFileStaysInsideDir(t *testing.T) {
	mm := newDownloadMM(t, "x", "payload")
	dir := t.TempDir()

	path, err := mm.DownloadFile(context.Background(), Attachment{ID: testFileID, Name: "../../escape.sh"}, dir)
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("saved to %s, outside %s", path, dir)
	}
}

func TestResolveAttachments(t *testing.T) {
	mm := newDownloadMM(t, "report.pdf", "x")
	ctx := context.Background()

	refs := map[string]string{
		"post ID":   testPostID,
		"permalink": "https://chat.acme.com/acme/pl/" + testPostID,
		"file ID":   testFileID, // not a post: falls back to the file
	}
	for name, ref := range refs {
		t.Run(name, func(t *testing.T) {
			files, err := mm.ResolveAttachments(ctx, ref)
			if err != nil {
				t.Fatalf("ResolveAttachments(%s): %v", ref, err)
			}
			if len(files) != 1 || files[0].ID != testFileID || files[0].Name != "report.pdf" {
				t.Errorf("got %+v, want the one report.pdf", files)
			}
		})
	}

	if _, err := mm.ResolveAttachments(ctx, "zzzz00000000000000000000aa"); err == nil {
		t.Error("expected an error for an ID that is neither a post nor a file")
	}
}

func TestXDGUserDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user-dirs.dirs")
	content := "# comment\nXDG_DESKTOP_DIR=\"$HOME/Escritorio\"\nXDG_DOWNLOAD_DIR=\"$HOME/Descargas\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := xdgUserDir(path, "XDG_DOWNLOAD_DIR", "/home/jane"), "/home/jane/Descargas"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := xdgUserDir(path, "XDG_MUSIC_DIR", "/home/jane"); got != "" {
		t.Errorf("missing key should give empty, got %q", got)
	}
}

func TestFormatSize(t *testing.T) {
	tests := map[int64]string{0: "0 B", 999: "999 B", 84710: "84.7 KB", 1_200_000: "1.2 MB"}
	for n, want := range tests {
		if got := FormatSize(n); got != want {
			t.Errorf("FormatSize(%d) = %q, want %q", n, got, want)
		}
	}
}
