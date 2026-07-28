package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// MaxFilesPerPost mirrors the Mattermost server limit on attachments per post.
// Checked locally so a batch of uploads fails before the first byte goes out.
const MaxFilesPerPost = 10

// ValidateAttachment reports whether a local path can be attached to a post.
// Exported so callers that queue files before sending (the TUI) reject bad paths
// at pick time with the same rules the upload itself applies.
func ValidateAttachment(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, not a file", path)
	}
	if info.Size() == 0 {
		return fmt.Errorf("%s is empty (Mattermost rejects empty files)", path)
	}
	return nil
}

// UploadFiles uploads local files to a channel and returns their file IDs in the
// same order, ready to be attached to a post via Post.FileIds. Uploaded files
// stay orphaned (and are eventually reaped by the server) until a post claims
// them, so a failure part-way through leaves no visible message.
func (mm *MM) UploadFiles(ctx context.Context, channelID string, paths []string) ([]string, error) {
	if channelID == "" {
		return nil, fmt.Errorf("channel is required to upload files")
	}
	if len(paths) > MaxFilesPerPost {
		return nil, fmt.Errorf("too many files: %d (Mattermost allows %d per message)", len(paths), MaxFilesPerPost)
	}

	ids := make([]string, 0, len(paths))
	for _, path := range paths {
		if err := ValidateAttachment(path); err != nil {
			return nil, err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("could not read %s: %w", path, err)
		}

		resp, _, err := mm.Client.UploadFile(ctx, data, channelID, filepath.Base(path))
		if err != nil {
			return nil, fmt.Errorf("could not upload %s: %w", path, err)
		}
		if len(resp.FileInfos) == 0 {
			return nil, fmt.Errorf("upload of %s returned no file info", path)
		}
		ids = append(ids, resp.FileInfos[0].Id)
	}
	return ids, nil
}
