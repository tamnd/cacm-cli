package cli

import (
	"github.com/spf13/cobra"
)

// feedCmd builds a command that fetches a CACM RSS/Atom feed at a fixed path
// relative to the configured BaseURL.
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

// techNewsCmd fetches the ACM TechNews feed from its own subdomain.
// Unlike the other feeds, TechNews lives at technews.acm.org, so FeedURL
// is used with an absolute URL rather than a path relative to BaseURL.
func (a *App) techNewsCmd() *cobra.Command {
	const techNewsURL = "https://technews.acm.org/feed/"
	return &cobra.Command{
		Use:   "technews",
		Short: "ACM TechNews newsletter digest (3x/week)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(20)
			a.progressf("fetching technews...")
			arts, err := a.client.FeedURL(cmd.Context(), techNewsURL, n)
			if err != nil {
				return codeError(exitError, err)
			}
			return a.renderOrEmpty(arts, len(arts))
		},
	}
}
