package cmd

import (
	"os/exec"

	"github.com/kairos-io/enki/pkg/action"
	"github.com/kairos-io/enki/pkg/config"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/mount-utils"
)

// NewBuildISOCmd returns a new instance of the build-iso subcommand and appends it to
// the root command.
func NewBuildUKICmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "build-uki SOURCE",
		Short: "Build a UKI artifact from a container image",
		Long: "Build a UKI artifact from a container image\n\n" +
			"SOURCE - should be provided as uri in following format <sourceType>:<sourceName>\n" +
			"    * <sourceType> - might be [\"dir\", \"file\", \"oci\", \"docker\"], as default is \"docker\"\n" +
			"    * <sourceName> - is path to file or directory, image name with tag version",
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// TODO: is this needed? Can we avoid it?
			// return CheckRoot()
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := exec.LookPath("mount")
			if err != nil {
				return err
			}
			mounter := mount.New(path)

			cfg, err := config.ReadConfigBuild(viper.GetString("config-dir"), cmd.Flags(), mounter)
			if err != nil {
				cfg.Logger.Errorf("Error reading config: %s\n", err)
			}

			//flags := cmd.Flags()

			// Set this after parsing of the flags, so it fails on parsing and prints usage properly
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true // Do not propagate errors down the line, we control them
			// TODO: Do we need to read a spec? The only input should be an image.
			// spec, err := config.ReadBuildISO(cfg, flags)
			// if err != nil {
			// 	cfg.Logger.Errorf("invalid install command setup %v", err)
			// 	return err
			// }

			if len(args) == 0 {
				cfg.Logger.Errorf("no image provided")
				return err
			}

			imgSource, err := v1.NewSrcFromURI(args[0])
			if err != nil {
				cfg.Logger.Errorf("not a valid rootfs source image argument: %s", args[0])
				return err
			}

			a := action.NewBuildUKIAction(cfg, imgSource)
			err = a.Run()
			if err != nil {
				cfg.Logger.Errorf(err.Error())
				return err
			}

			return nil
		},
	}

	// TODO: Implement these? We can keep it simple by using the artifact name
	// from the image's os-release file.
	// c.Flags().StringP("name", "n", "", "Basename of the generated ISO file")
	// c.Flags().StringP("output", "o", "", "Output directory (defaults to current directory)")
	// c.Flags().Bool("date", false, "Adds a date suffix into the generated ISO file")
	// c.Flags().String("label", "", "Label of the ISO volume")
	return c
}

func init() {
	rootCmd.AddCommand(NewBuildUKICmd())
}
