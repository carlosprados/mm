package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testDoc = "---\nname: mm\n---\n\n# mm\n"

func TestInstallSkillWritesAndStamps(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mm")
	file, err := installSkill(dir, testDoc, "v1.2.3", false)
	if err != nil {
		t.Fatalf("installSkill: %v", err)
	}
	got, _ := os.ReadFile(file)
	if !strings.HasPrefix(string(got), testDoc) || !strings.Contains(string(got), "<!-- installed by mm v1.2.3 -->") {
		t.Errorf("unexpected content:\n%s", got)
	}
}

// Upgrading the binary changes only the stamp: that is not a difference worth
// refusing over.
func TestInstallSkillUpToDateIgnoresStamp(t *testing.T) {
	dir := t.TempDir()
	if _, err := installSkill(dir, testDoc, "v1.0.0", false); err != nil {
		t.Fatal(err)
	}
	file, err := installSkill(dir, testDoc, "v2.0.0", false)
	if err != nil {
		t.Fatalf("installSkill: %v", err)
	}
	if file != "" {
		t.Errorf("expected up to date, got a write to %s", file)
	}
}

func TestInstallSkillRefusesToClobber(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(file, []byte("edited by hand"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := installSkill(dir, testDoc, "v1", false); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected a refusal mentioning --force, got %v", err)
	}
	if b, _ := os.ReadFile(file); string(b) != "edited by hand" {
		t.Error("file was changed without --force")
	}
	if _, err := installSkill(dir, testDoc, "v1", true); err != nil {
		t.Fatalf("with --force: %v", err)
	}
	if b, _ := os.ReadFile(file); !strings.HasPrefix(string(b), testDoc) {
		t.Error("--force did not replace the file")
	}
}

// A symlink into a checkout must never be written through.
func TestInstallSkillRefusesSymlinks(t *testing.T) {
	checkout := t.TempDir()
	original := filepath.Join(checkout, "SKILL.md")
	if err := os.WriteFile(original, []byte("repo copy"), 0o644); err != nil {
		t.Fatal(err)
	}

	fileLink := t.TempDir()
	if err := os.Symlink(original, filepath.Join(fileLink, "SKILL.md")); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	dirLink := filepath.Join(t.TempDir(), "mm")
	if err := os.Symlink(checkout, dirLink); err != nil {
		t.Fatal(err)
	}

	for name, dir := range map[string]string{"file link": fileLink, "dir link": dirLink} {
		if _, err := installSkill(dir, testDoc, "v1", false); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Errorf("%s: expected a symlink refusal, got %v", name, err)
		}
	}
	if b, _ := os.ReadFile(original); string(b) != "repo copy" {
		t.Error("the checkout's SKILL.md was written through a symlink")
	}
}

func TestInstallSkillNeedsADocument(t *testing.T) {
	if _, err := installSkill(t.TempDir(), "  \n", "v1", false); err == nil {
		t.Error("expected an error for an empty document")
	}
}

// SKILL.md is what agents learn mm from: a command it does not mention is a
// command they will not use, or will improvise. Commands that are not for
// agents are listed here on purpose.
func TestSkillMentionsEveryCommand(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	skip := map[string]bool{"help": true, "completion": true, "version": true, "logout": true, "skill": true}
	for _, c := range rootCmd.Commands() {
		if skip[c.Name()] {
			continue
		}
		if !strings.Contains(string(doc), "mm "+c.Name()) {
			t.Errorf("SKILL.md never mentions `mm %s`", c.Name())
		}
		for _, sub := range c.Commands() {
			if want := "mm " + c.Name() + " " + sub.Name(); !strings.Contains(string(doc), want) {
				t.Errorf("SKILL.md never mentions `%s`", want)
			}
		}
	}
}
