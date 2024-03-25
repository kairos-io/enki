package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kairos-io/enki/pkg/action"
	"github.com/kairos-io/enki/pkg/config"
	"github.com/kairos-io/enki/pkg/constants"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewBuildUKICmd returns a new instance of the build-uki subcommand and appends it to
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
			artifact, err := cmd.Flags().GetString("output-type")
			if err != nil {
				return err
			}
			if artifact != string(constants.DefaultOutput) && artifact != string(constants.IsoOutput) && artifact != string(constants.ContainerOutput) {
				return fmt.Errorf("invalid output type: %s", artifact)
			}

			overlayRootfs, _ := cmd.Flags().GetString("overlay-rootfs")
			if overlayRootfs != "" {
				// Check if overlay dir exists by doing an os.stat
				// If it does not exist, return an error
				ol, err := os.Stat(overlayRootfs)
				if err != nil {
					return fmt.Errorf("overlay-rootfs directory does not exist: %s", overlayRootfs)
				}
				if !ol.IsDir() {
					return fmt.Errorf("overlay-rootfs is not a directory: %s", overlayRootfs)
				}

				// Transform it into absolute path
				absolutePath, err := filepath.Abs(overlayRootfs)
				if err != nil {
					viper.Set("overlay-rootfs", absolutePath)
				}
			}
			overlayIso, _ := cmd.Flags().GetString("overlay-iso")
			if overlayIso != "" {
				// Check if overlay dir exists by doing an os.stat
				// If it does not exist, return an error
				ol, err := os.Stat(overlayIso)
				if err != nil {
					return fmt.Errorf("overlay directory does not exist: %s", overlayIso)
				}
				if !ol.IsDir() {
					return fmt.Errorf("overlay is not a directory: %s", overlayIso)
				}

				// Check if we are setting a different artifact and overlay-iso is set
				if artifact != string(constants.IsoOutput) {
					return fmt.Errorf("overlay-iso is only supported for iso artifacts")
				}

				// Transform it into absolute path
				absolutePath, err := filepath.Abs(overlayIso)
				if err != nil {
					viper.Set("overlay-iso", absolutePath)
				}
			}

			// Check if the keys directory exists
			keysDir, _ := cmd.Flags().GetString("keys")
			_, err = os.Stat(keysDir)
			if err != nil {
				return fmt.Errorf("keys directory does not exist: %s", keysDir)
			}
			// Check if the keys directory contains the required files
			requiredFiles := []string{"db.crt", "db.der", "db.key", "db.auth", "KEK.der", "KEK.auth", "PK.der", "PK.auth", "tpm2-pcr-private.pem"}
			for _, file := range requiredFiles {
				_, err = os.Stat(filepath.Join(keysDir, file))
				if err != nil {
					return fmt.Errorf("keys directory does not contain required file: %s", file)
				}
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
			outputDir, _ := flags.GetString("output-dir")
			keysDir, _ := flags.GetString("keys")
			outputType, _ := flags.GetString("output-type")
			a := action.NewBuildUKIAction(cfg, imgSource, outputDir, keysDir, outputType)
			err = a.Run()
			if err != nil {
				cfg.Logger.Errorf(err.Error())
				return err
			}

			return nil
		},
	}

	c.Flags().StringP("output-dir", "d", ".", "Output dir for artifact")
	c.Flags().StringP("output-type", "t", string(constants.DefaultOutput), fmt.Sprintf("Artifact output type [%s]", strings.Join(constants.OutPutTypes(), ", ")))
	c.Flags().StringP("overlay-rootfs", "o", "", "Dir with files to be applied to the system rootfs.\nAll the files under this dir will be copied into the rootfs of the uki respecting the directory structure under the dir.")
	c.Flags().StringP("overlay-iso", "i", "", "Dir with files to be copied to the Iso rootfs.")
	c.Flags().StringP("boot-branding", "", "Kairos", "Boot title branding")
	c.Flags().BoolP("include-version-in-config", "", false, "Include the OS version in the .config file")
	c.Flags().BoolP("include-cmdline-in-config", "", false, "Include the cmdline in the .config file. Only the extra values are included.")
	c.Flags().StringSliceP("extra-cmdline", "c", []string{}, "Add extra efi files with this cmdline. This creates a base efi with the default cmdline and extra efi files with the default+provided cmdline.")
	c.Flags().StringP("extend-cmdline", "x", "", "Extend the default cmdline with this parameters. This creates a single efi entry with the default+provided cmdline.")
	c.Flags().StringP("keys", "k", "", "Directory with the signing keys")
	c.Flags().StringP("default-entry", "e", "", "Default entry selected in the boot menu.\nSupported glob wildcard patterns are \"?\", \"*\", and \"[...]\".\nIf not selected, the default entry with install-mode is selected.")
	c.Flags().Int64P("efi-size-warn", "", 1024, "EFI file size warning threshold in megabytes. Default is 1024.")
	c.MarkFlagRequired("keys")
	// Mark some flags as mutually exclusive
	c.MarkFlagsMutuallyExclusive([]string{"extra-cmdline", "extend-cmdline"}...)
	viper.BindPFlags(c.Flags())
	return c
}

func init() {
	rootCmd.AddCommand(NewBuildUKICmd())
}
