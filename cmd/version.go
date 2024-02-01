package cmd

import (
	"fmt"
	"github.com/kairos-io/enki/internal/version"
	"github.com/spf13/cobra"
)

func NewVersionCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "version",
		Short: "Print enki version",
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			// Set this after parsing of the flags, so it fails on parsing and prints usage properly
			cobraCmd.SilenceUsage = true
			cobraCmd.SilenceErrors = true // Do not propagate errors down the line, we control them
			var long bool
			long, _ = cobraCmd.Flags().GetBool("long")
			if long {
				fmt.Printf("%+v\n", version.Get())
			} else {
				fmt.Println(version.VERSION)
			}
			return nil
		},
	}
	c.Flags().BoolP("long", "l", false, "Output long version information")
	return c
}

func init() {
	rootCmd.AddCommand(NewVersionCmd())
}
