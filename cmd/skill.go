package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// SkillDoc is SKILL.md, embedded into the binary by main.
var SkillDoc string

// stampRe matches the marker appended on install, so an installed copy can be
// compared with the embedded one without the version line getting in the way.
var stampRe = regexp.MustCompile(`(?m)\n<!-- installed by mm [^>]* -->\n?$`)

var (
	skillDir     string
	skillProject bool
	skillForce   bool
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Install the agent skill that teaches an AI to drive mm",
	Long: "mm ships with a skill document for coding agents (Claude Code and anything\n" +
		"else that reads a skills directory): commands, JSON shapes, the rules for\n" +
		"writing to real people, and the errors. It is embedded in this binary:\n\n" +
		"  mm skill install            into ~/.claude/skills/mm/SKILL.md\n" +
		"  mm skill install --project  into ./.claude/skills/mm/SKILL.md\n" +
		"  mm skill show               print it\n" +
		"  mm skill path               where install would write",
}

var skillShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the embedded skill document",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprint(cmd.OutOrStdout(), SkillDoc)
		return nil
	},
}

var skillPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print where the skill would be installed",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := skillTarget(skillDir, skillProject)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), filepath.Join(dir, "SKILL.md"))
		return nil
	},
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Write the embedded skill where an agent will find it",
	Long: "Writes the embedded skill document into a skills directory. Re-run it after\n" +
		"upgrading mm to keep the skill in step with the binary.\n\n" +
		"Refuses to overwrite a file that differs from the embedded one, or a\n" +
		"symlink, unless --force: a symlink into a checkout is how mm itself is\n" +
		"developed, and writing through it would edit the repository.",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := skillTarget(skillDir, skillProject)
		if err != nil {
			return err
		}
		file, err := installSkill(dir, SkillDoc, version, skillForce)
		if err != nil {
			return err
		}
		if file == "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%s is already up to date\n", filepath.Join(dir, "SKILL.md"))
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "installed %s\n\nIn Claude Code it loads by name, \"mm\".\n", file)
		return nil
	},
}

// skillTarget is the directory holding SKILL.md: --dir as given, the project's
// .claude/skills/mm with --project, otherwise the personal one. Without a home
// directory that last one is an error rather than a path relative to wherever
// the command happens to run.
func skillTarget(dir string, project bool) (string, error) {
	switch {
	case dir != "":
		return dir, nil
	case project:
		return filepath.Join(".claude", "skills", "mm"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("no home directory to find ~/.claude/skills in: set HOME, or use --project or --dir")
	}
	return filepath.Join(home, ".claude", "skills", "mm"), nil
}

// installSkill writes doc, stamped with the version, to dir/SKILL.md and
// returns the path written, or "" when the installed copy already matches.
func installSkill(dir, doc, ver string, force bool) (string, error) {
	if strings.TrimSpace(doc) == "" {
		return "", fmt.Errorf("this build carries no embedded skill document")
	}
	file := filepath.Join(dir, "SKILL.md")
	body := strings.TrimRight(doc, "\n") + fmt.Sprintf("\n\n<!-- installed by mm %s -->\n", ver)

	// Lstat, not Stat: a symlink must be seen as one, or writing follows it.
	// The directory may be the link rather than the file.
	if st, err := os.Lstat(dir); err == nil && st.Mode()&os.ModeSymlink != 0 {
		dest, _ := os.Readlink(dir)
		if !force {
			return "", fmt.Errorf("%s is a symlink to %s: installing would edit the SKILL.md there; if that is a checkout of mm there is nothing to do, otherwise pass --force to replace the link with a directory", dir, dest)
		}
		if err := os.Remove(dir); err != nil {
			return "", fmt.Errorf("could not remove symlink %s: %w", dir, err)
		}
	}
	if st, err := os.Lstat(file); err == nil {
		link := st.Mode()&os.ModeSymlink != 0
		current, _ := os.ReadFile(file)
		same := stampRe.ReplaceAllString(string(current), "") == stampRe.ReplaceAllString(body, "")
		switch {
		case same && !link:
			return "", nil
		case link && !force:
			dest, _ := os.Readlink(file)
			return "", fmt.Errorf("%s is a symlink to %s: overwriting would edit that file; pass --force to replace it with a copy", file, dest)
		case !force:
			return "", fmt.Errorf("%s exists and differs from this build's copy; pass --force to replace it", file)
		}
		if link {
			if err := os.Remove(file); err != nil {
				return "", fmt.Errorf("could not remove symlink %s: %w", file, err)
			}
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("could not create %s: %w", dir, err)
	}
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		return "", fmt.Errorf("could not write %s: %w", file, err)
	}
	return file, nil
}

func init() {
	for _, c := range []*cobra.Command{skillInstallCmd, skillPathCmd} {
		c.Flags().StringVar(&skillDir, "dir", "", "Use this directory instead")
		c.Flags().BoolVar(&skillProject, "project", false, "Use ./.claude/skills/mm instead of your home")
	}
	skillInstallCmd.Flags().BoolVar(&skillForce, "force", false, "Replace an existing, different or symlinked SKILL.md")
	skillCmd.AddCommand(skillInstallCmd, skillShowCmd, skillPathCmd)
	rootCmd.AddCommand(skillCmd)
}
