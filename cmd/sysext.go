package cmd

import (
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/kairos-io/enki/pkg/config"
	"github.com/kairos-io/kairos-sdk/sysext"
	"github.com/kairos-io/kairos-sdk/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func NewSysextCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "sysext NAME CONTAINER",
		Short: "Generate a sysextension from the last layer of the given CONTAINER",
		Args:  cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			arch := viper.GetString("arch")
			if arch != "amd64" && arch != "arm64" {
				return fmt.Errorf("unsupported architecture: %s", arch)
			}
			return nil
		},
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			// Set this after parsing of the flags, so it fails on parsing and prints usage properly
			cobraCmd.SilenceUsage = true
			// we log the errors with our nice logger so stop cobra from logging them, just let it return the exit codes
			cobraCmd.SilenceErrors = true

			cfg, err := config.ReadConfigBuild(viper.GetString("config-dir"), cobraCmd.Flags())
			if err != nil {
				return err
			}

			name := args[0]
			_, err = os.Stat(fmt.Sprintf("%s.sysext.raw", name))
			if err == nil {
				_ = os.Remove(fmt.Sprintf("%s.sysext.raw", name))
			}
			cfg.Logger.Logger.Info().Msg("🚀 Start sysext creation")

			dir, _ := os.MkdirTemp("", "")
			defer func(path string) {
				err := os.RemoveAll(path)
				if err != nil {
					cfg.Logger.Logger.Error().Str("dir", dir).Err(err).Msg("⛔ removing dir")
				}
			}(dir)
			cfg.Logger.Logger.Debug().Str("dir", dir).Msg("creating directory")
			// Get the image struct
			cfg.Logger.Logger.Info().Msg("💿 Getting image info")

			platform := fmt.Sprintf("linux/%s", viper.Get("arch"))
			image, err := utils.GetImage(args[1], platform, nil, nil)
			if err != nil {
				cfg.Logger.Logger.Error().Str("image", args[1]).Err(err).Msg("⛔ getting image")
				return err
			}
			// Only for sysext, confext not supported yet
			AllowList := regexp.MustCompile(`^usr/*|^/usr/*`)
			// extract the files into the temp dir
			cfg.Logger.Logger.Info().Msg("📤 Extracting archives from image layer")
			err = sysext.ExtractFilesFromLastLayer(image, dir, cfg.Logger, AllowList)
			if err != nil {
				cfg.Logger.Logger.Error().Str("image", args[1]).Err(err).Msg("⛔ extracting layer")
			}

			// Now create the file that tells systemd that this is a sysext!
			err = os.MkdirAll(filepath.Join(dir, "/usr/lib/extension-release.d/"), os.ModeDir|os.ModePerm)
			if err != nil {
				cfg.Logger.Logger.Error().Str("dir", filepath.Join(dir, "/usr/lib/extension-release.d/")).Err(err).Msg("⛔ creating dir")
			}

			extensionData := "ID=_any\nARCHITECTURE=x86-64"

			if viper.Get("arch") == "arm64" {
				extensionData = "ID=_any\nARCHITECTURE=arm64"
			}

			// If the extension ships any service files, we want this so systemd is reloaded and the service available immediately
			if viper.GetBool("service-reload") {
				extensionData = fmt.Sprintf("%s\nEXTENSION_RELOAD_MANAGER=1", extensionData)
			}
			err = os.WriteFile(filepath.Join(dir, "/usr/lib/extension-release.d/", fmt.Sprintf("extension-release.%s", name)), []byte(extensionData), os.ModePerm)
			if err != nil {
				cfg.Logger.Logger.Error().Str("file", fmt.Sprintf("extension-release.%s", name)).Err(err).Msg("⛔ creating releasefile")
			}

			cfg.Logger.Logger.Info().Msg("📦 Packing sysext into raw image")
			// Call systemd-repart to create the sysext based off the files
			command := exec.Command(
				"systemd-repart",
				"--make-ddi=sysext",
				"--image-policy=root=verity+signed+absent:usr=verity+signed+absent",
				fmt.Sprintf("--architecture=%s", viper.Get("arch")),
				// Having a fixed predictable seed makes the Image UUID be always the same if the inputs are the same,
				// so its a reproducible image. So getting the same files and same cert/key should produce a reproducible image always
				// Another layer to verify images, even if its a manual check, we make it easier
				fmt.Sprintf("--seed=%s", uuid.NewV5(uuid.NamespaceDNS, "kairos-sysext")),
				fmt.Sprintf("--copy-source=%s", dir),
				fmt.Sprintf("%s.sysext.raw", name), // output sysext image
				fmt.Sprintf("--private-key=%s", viper.Get("private-key")),
				fmt.Sprintf("--certificate=%s", viper.Get("certificate")),
			)
			out, err := command.CombinedOutput()
			cfg.Logger.Logger.Debug().Str("output", string(out)).Msg("building sysext")
			if err != nil {
				cfg.Logger.Logger.Error().Err(err).Str("command", strings.Join(command.Args, " ")).Msg("⛔ building sysext")
				return err
			}

			cfg.Logger.Logger.Info().Str("output", fmt.Sprintf("%s.sysext.raw", name)).Msg("🎉 Done sysext creation")
			return nil
		},
	}
	c.Flags().String("private-key", "", "Private key to sign the sysext with")
	c.Flags().String("certificate", "", "Certificate to sign the sysext with")
	c.Flags().Bool("service-reload", false, "Make systemctl reload the service when loading the sysext. This is useful for sysext that provide systemd service files.")
	c.Flags().String("arch", "amd64", "Arch to get the image from and build the sysext for. Accepts amd64 and arm64 values.")
	_ = c.MarkFlagRequired("private-key")
	_ = c.MarkFlagRequired("certificate")

	err := viper.BindPFlags(c.Flags())
	if err != nil {
		return nil
	}

	return c
}

func init() {
	rootCmd.AddCommand(NewSysextCmd())
}
