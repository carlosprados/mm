package main

import (
	_ "embed"

	"github.com/carlosprados/mm/cmd"
)

// The agent skill travels inside the binary, so `mm skill install` works for
// someone who installed a release and has no checkout of this repository. The
// embed lives here because //go:embed cannot reach outside its own package
// directory, and SKILL.md belongs at the repository root where people read it.
//
//go:embed SKILL.md
var skillDoc string

func main() {
	cmd.SkillDoc = skillDoc
	cmd.Execute()
}
