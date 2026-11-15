package main

import (
	"fmt"

	"github.com/JungHoonGhae/tossinvest-cli/internal/skill"
	"github.com/spf13/cobra"
)

// newSkillCmd builds `tossctl skill`: manage the tossctl agent skill that ships
// embedded in this binary.
//
// The skill is a guide that teaches an agent (Claude Code and other MCP/CLI
// clients) how to drive tossctl safely — discover operations, set up auth,
// respect the read-first + trading-gated model, never expose real account data.
// It is version-controlled at skills/tossctl/ and installable with a curl|sh
// one-liner, but the project wanted a CLI-native path too. `skill install` is
// that path: it copies the embedded copy into the Claude skills directory, works
// offline, and always installs the skill that matches the running binary.
func newSkillCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "skill",
		Short:       "Manage the embedded tossctl agent skill",
		Annotations: map[string]string{"source": "local"},
		Long: "The tossctl agent skill teaches an agent how to drive tossctl (operation " +
			"discovery, auth, the read-first + trading-gated safety model, and never " +
			"exposing real account data). It ships embedded in this binary.\n\n" +
			"`tossctl skill install` copies it into your Claude skills directory " +
			"(~/.claude/skills, or $CLAUDE_SKILLS_DIR). Restart your Claude client " +
			"afterward to pick it up.",
	}
	cmd.AddCommand(newSkillInstallCmd(opts))
	return cmd
}

func newSkillInstallCmd(_ *rootOptions) *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:         "install",
		Short:       "Install the tossctl agent skill into a Claude skills directory",
		Annotations: map[string]string{"source": "local"},
		Long: "Write the embedded tossctl skill into your Claude skills directory so an " +
			"agent can use it. Target: --dir, else $CLAUDE_SKILLS_DIR, else " +
			"~/.claude/skills. An existing install is overwritten, so run it again after " +
			"upgrading tossctl to refresh the skill.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			skillsDir := dir
			if skillsDir == "" {
				resolved, err := skill.DefaultDir()
				if err != nil {
					return fmt.Errorf("resolve skills directory: %w", err)
				}
				skillsDir = resolved
			}

			written, err := skill.Install(skillsDir)
			if err != nil {
				return fmt.Errorf("install skill: %w", err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Installed the tossctl skill into %s:\n", skill.InstalledDir(skillsDir))
			for _, path := range written {
				fmt.Fprintf(out, "  %s\n", path)
			}
			fmt.Fprintln(out, "\nRestart your Claude client (or re-scan skills) to pick up 'tossctl'.")
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Skills directory to install into (default: $CLAUDE_SKILLS_DIR or ~/.claude/skills)")
	return cmd
}
