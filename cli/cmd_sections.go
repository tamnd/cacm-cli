package cli

import (
	"github.com/spf13/cobra"
	"github.com/tamnd/cacm-cli/cacm"
)

func (a *App) sectionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sections",
		Short: "List all known CACM feed sections and their URLs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			secs := cacm.KnownSections()
			return a.renderOrEmpty(secs, len(secs))
		},
	}
}
