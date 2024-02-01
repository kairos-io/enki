package cmd

import (
	"fmt"
	"github.com/kairos-io/enki/pkg/action"
	"github.com/kairos-io/enki/pkg/config"
	"github.com/kairos-io/enki/pkg/constants"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"strings"
)

// NewBuildISOCmd returns a new instance of the build-iso subcommand and appends it to
// the root command.
func NewBuildUKICmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "build-uki SourceImage",
		Short: "Build a UKI artifact from a container image",
		Long: "Build a UKI artifact from a container image\n\n" +
			"SourceImage - should be provided as uri in following format <sourceType>:<sourceName>\n" +
			"    * <sourceType> - might be [\"dir\", \"file\", \"oci\", \"docker\"], as default is \"docker\"\n" +
			"    * <sourceName> - is path to file or directory, image name with tag version" +
			"The following files are expected inside the keys directory:\n" +
			"    - DB.crt\n" +
			"    - DB.der\n" +
			"    - DB.key\n" +
			"    - DB.auth\n" +
			"    - KEK.der\n" +
			"    - KEK.auth\n" +
			"    - PK.der\n" +
			"    - PK.auth\n" +
			"    - tpm2-pcr-private.pem\n",
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			artifact, err := cmd.Flags().GetString("artifact")
			if err != nil {
				return err
			}
			if artifact != string(constants.DefaultOutput) && artifact != string(constants.IsoOutput) && artifact != string(constants.ContainerOutput) {
				return fmt.Errorf("invalid artifact type: %s", artifact)
			}

			return CheckRoot()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.ReadConfigBuild(viper.GetString("config-dir"), cmd.Flags())
			if err != nil {
				cfg.Logger.Errorf("Error reading config: %s\n", err)
			}

			// Set this after parsing of the flags, so it fails on parsing and prints usage properly
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true // Do not propagate errors down the line, we control them

			if len(args) == 0 {
				cfg.Logger.Errorf("no image provided")
				return err
			}

			imgSource, err := v1.NewSrcFromURI(args[0])
			if err != nil {
				cfg.Logger.Errorf("not a valid rootfs source image argument: %s", args[0])
				return err
			}

			flags := cmd.Flags()
			outputDir, _ := flags.GetString("output")
			keysDir, _ := flags.GetString("keys")
			artifact, _ := flags.GetString("artifact")
			a := action.NewBuildUKIAction(cfg, imgSource, outputDir, keysDir, artifact)
			err = a.Run()
			if err != nil {
				cfg.Logger.Errorf(err.Error())
				return err
			}

			return nil
		},
	}

	c.Flags().StringP("output", "o", ".", "Output dir for artifact")
	c.Flags().StringSliceP("cmdline", "c", []string{}, "Command line to ")
	c.Flags().StringP("keys", "k", "", "Directory with the signing keys")
	c.MarkFlagRequired("keys")
	c.Flags().StringP("artifact", "a", string(constants.DefaultOutput), fmt.Sprintf("Artifact type [%s]", strings.Join(constants.OutPutTypes(), ", ")))
	return c
}

func init() {
	rootCmd.AddCommand(NewBuildUKICmd())
}
