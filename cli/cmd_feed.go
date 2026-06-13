package cli

import (
	"github.com/spf13/cobra"
)

// feedCmd builds a command that fetches a CACM RSS/Atom feed at a fixed path.
func (a *App) feedCmd(use, short, path string, defaultLimit int) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(defaultLimit)
			a.progressf("fetching %s...", use)
			arts, err := a.client.Feed(cmd.Context(), path, n)
			if err != nil {
				return codeError(exitError, err)
			}
			return a.renderOrEmpty(arts, len(arts))
		},
	}
}
