package client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

// lastAttachmentScanWindow is how many recent posts are scanned when looking
// for the latest message that carries attachments.
const lastAttachmentScanWindow = 50

// Attachment is a file attached to a post, as described by the server.
type Attachment struct {
	ID       string
	PostID   string
	Name     string
	Size     int64
	MimeType string
}

// IsImage reports whether the attachment can be rendered as an image.
func (a Attachment) IsImage() bool {
	return strings.HasPrefix(a.MimeType, "image/")
}

// AttachmentsOf reads a post's attachments from the metadata the server already
// embeds in every post it returns, so listing them costs no extra request. If
// the metadata is missing, the bare IDs are kept so a download still works.
func AttachmentsOf(p *model.Post) []Attachment {
	if len(p.FileIds) == 0 {
		return nil
	}
	if p.Metadata != nil && len(p.Metadata.Files) > 0 {
		out := make([]Attachment, 0, len(p.Metadata.Files))
		for _, fi := range p.Metadata.Files {
			out = append(out, toAttachment(fi, p.Id))
		}
		return out
	}
	out := make([]Attachment, 0, len(p.FileIds))
	for _, id := range p.FileIds {
		out = append(out, Attachment{ID: id, PostID: p.Id})
	}
	return out
}

func toAttachment(fi *model.FileInfo, postID string) Attachment {
	if fi.PostId != "" {
		postID = fi.PostId
	}
	return Attachment{ID: fi.Id, PostID: postID, Name: fi.Name, Size: fi.Size, MimeType: fi.MimeType}
}

// ResolveAttachments turns a reference into the attachments it points at: a
// permalink or post ID yields all files of that post, a file ID yields that one
// file. Post and file IDs look alike, so a bare ID is tried as a post first.
func (mm *MM) ResolveAttachments(ctx context.Context, ref string) ([]Attachment, error) {
	id, err := ParsePostRef(ref)
	if err != nil {
		return nil, fmt.Errorf("%q is not a post ID, permalink or file ID", ref)
	}
	if post, _, err := mm.Client.GetPost(ctx, id, ""); err == nil {
		files, err := mm.PostAttachments(ctx, post)
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("message %s has no attachments", id)
		}
		return files, nil
	}
	fi, _, err := mm.Client.GetFileInfo(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("no message or file with ID %s: %w", id, err)
	}
	return []Attachment{toAttachment(fi, "")}, nil
}

// PostAttachments returns a post's attachments, asking the server for their
// descriptions only when the post came without metadata.
func (mm *MM) PostAttachments(ctx context.Context, p *model.Post) ([]Attachment, error) {
	files := AttachmentsOf(p)
	if len(files) == 0 || files[0].Name != "" {
		return files, nil
	}
	infos, _, err := mm.Client.GetFileInfosForPost(ctx, p.Id, "")
	if err != nil {
		return nil, fmt.Errorf("could not list attachments of %s: %w", p.Id, err)
	}
	out := make([]Attachment, 0, len(infos))
	for _, fi := range infos {
		out = append(out, toAttachment(fi, p.Id))
	}
	return out, nil
}

// LastAttachments returns the attachments of the most recent message in the
// target that has any: the "they just sent me a file" case.
func (mm *MM) LastAttachments(ctx context.Context, t Target) ([]Attachment, error) {
	channelID, err := mm.ResolveChannelID(ctx, t)
	if err != nil {
		return nil, err
	}
	posts, _, err := mm.Client.GetPostsForChannel(ctx, channelID, 0, lastAttachmentScanWindow, "", false, false)
	if err != nil {
		return nil, fmt.Errorf("could not fetch posts: %w", err)
	}
	for _, id := range posts.Order { // newest first
		if p := posts.Posts[id]; len(p.FileIds) > 0 {
			return mm.PostAttachments(ctx, p)
		}
	}
	return nil, fmt.Errorf("no message with attachments in the last %d posts", lastAttachmentScanWindow)
}

// DownloadFile saves an attachment into dir and returns the path written. The
// server-supplied name is reduced to its base name, so it cannot escape dir,
// and an existing file is never overwritten: "report (1).pdf" is used instead.
// The content is written to a temporary file first and only then linked under
// its final name, so an interrupted download never leaves a truncated file
// that looks complete.
func (mm *MM) DownloadFile(ctx context.Context, a Attachment, dir string) (string, error) {
	if dir == "" {
		dir = DefaultDownloadDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("could not create %s: %w", dir, err)
	}

	data, _, err := mm.Client.GetFile(ctx, a.ID)
	if err != nil {
		return "", fmt.Errorf("could not download %s: %w", displayName(a), err)
	}

	tmp, err := os.CreateTemp(dir, ".mm-*.part")
	if err != nil {
		return "", fmt.Errorf("could not write into %s: %w", dir, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("could not write %s: %w", tmp.Name(), err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("could not write %s: %w", tmp.Name(), err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return "", fmt.Errorf("could not set permissions on %s: %w", tmp.Name(), err)
	}

	return claimName(tmp.Name(), dir, SafeFileName(a.Name, a.ID))
}

// claimName moves src to the first free "name", "name (1)", … in dir. A hard
// link fails atomically when the name is taken, so two downloads racing for the
// same name cannot clobber each other. Filesystems without hard links fall back
// to a check-then-rename.
func claimName(src, dir, name string) (string, error) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := 0; n < 1000; n++ {
		candidate := name
		if n > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", stem, n, ext)
		}
		dst := filepath.Join(dir, candidate)

		err := os.Link(src, dst)
		if err == nil {
			return dst, nil
		}
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if _, statErr := os.Lstat(dst); statErr == nil {
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			return "", fmt.Errorf("could not save %s: %w", dst, err)
		}
		return dst, nil
	}
	return "", fmt.Errorf("could not find a free name for %s in %s", name, dir)
}

// SafeFileName reduces a server-supplied file name to something that can only
// land inside the target directory. A name that is empty after cleaning falls
// back to the file ID.
func SafeFileName(name, fileID string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || name == "/" {
		return "file-" + fileID
	}
	return name
}

func displayName(a Attachment) string {
	if a.Name != "" {
		return a.Name
	}
	return a.ID
}

// DefaultDownloadDir is the user's download directory as configured for the
// desktop (XDG_DOWNLOAD_DIR, which on a Spanish desktop is ~/Descargas), then
// ~/Downloads, then the current directory.
func DefaultDownloadDir() string {
	home, _ := os.UserHomeDir()
	if dir := os.Getenv("XDG_DOWNLOAD_DIR"); dir != "" {
		return dir
	}
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" && home != "" {
		configHome = filepath.Join(home, ".config")
	}
	if dir := xdgUserDir(filepath.Join(configHome, "user-dirs.dirs"), "XDG_DOWNLOAD_DIR", home); dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	if home != "" {
		dir := filepath.Join(home, "Downloads")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return "."
}

// xdgUserDir reads one entry of a user-dirs.dirs file, whose lines look like
// XDG_DOWNLOAD_DIR="$HOME/Descargas".
func xdgUserDir(path, key, home string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		value, ok := strings.CutPrefix(line, key+"=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		if rest, ok := strings.CutPrefix(value, "$HOME"); ok {
			if home == "" {
				return ""
			}
			value = home + rest
		}
		if !filepath.IsAbs(value) {
			return ""
		}
		return value
	}
	return ""
}

// FormatSize renders a byte count for humans: 512 B, 84.7 KB, 1.2 MB.
func FormatSize(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}
