package cmd

import (
	"fmt"

	"github.com/kairos-io/enki/pkg/action"
	"github.com/spf13/cobra"
)

// NewFindMatchingFrameworkCmd returns a new instance of the build-iso subcommand and appends it to
// the root command.
func NewFindMatchingFrameworkCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "find-matching-framework",
		Short: "Finds the framework image that matches the current OS best",
		Long: "Finds the framework image that matches the current OS best\n" +
			"This is best effort and might return a framework image that doesn't work.\n" +
			"It is expected to work with the official Kairos flavors.",
		Args: cobra.ExactArgs(0),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			//return CheckRoot() // TODO: Do we need this?
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			a := action.NewFindMatchingFrameworkAction()
			result, err := a.Run()
			if err != nil {
				return err
			}

			fmt.Println(result)

			return nil
		},
	}

	return c
}

func init() {
	rootCmd.AddCommand(NewFindMatchingFrameworkCmd())
}
