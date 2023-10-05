package cmd

import (
	"fmt"

	"github.com/kairos-io/enki/pkg/action"
	"github.com/spf13/cobra"
)

// NewDockerfilCmd generates a dockerfile which can be used to build a kairos
// image out of the provided one.
func NewDockerfileCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "dockerfile",
		Short: "Create a dockerfile that builds a Kairos image from the provided one",
		Long: "Create a dockerfile that builds a Kairos image from the provided one\n\n" +
			"The base image can be specified either as a directory where the image has been extracted or as an image uri.\n" +
			"This is best effort. Enki will try to detect the distribution and add the necessary bits to convert it to a Kairos image",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			//return CheckRoot() // TODO: Do we need root?
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Set this after parsing of the flags, so it fails on parsing and prints usage properly
			cmd.SilenceUsage = true
			//cmd.SilenceErrors = true // Do not propagate errors down the line, we control them

			rootfsDir, err := cmd.Flags().GetString("rootfs-dir")
			if err != nil {
				return err
			}

			baseImageURI, err := cmd.Flags().GetString("base-image-uri")
			if err != nil {
				return err
			}

			frameworkImage, err := cmd.Flags().GetString("framework-image")
			if err != nil {
				return err
			}

			osReleaseVarsPath, err := cmd.Flags().GetString("os-release-vars-path")
			if err != nil {
				return err
			}

			a := action.DockerfileAction{
				RootFSPath:        rootfsDir,
				BaseImageURI:      baseImageURI,
				FrameworkImage:    frameworkImage,
				OSReleaseVarsPath: osReleaseVarsPath,
			}
			dockerfile, err := a.Run()
			if err != nil {
				return err
			}

			fmt.Println(dockerfile)

			return nil
		},
	}

	return c
}

func init() {
	c := NewDockerfileCmd()
	rootCmd.AddCommand(c)
	c.Flags().StringP("rootfs-dir", "r", "", "the directory containing the extracted base image rootfs")
	c.Flags().StringP("base-image-uri", "b", "", "the URI of the base image")
	c.Flags().StringP("framework-image", "i", "", "the URI of the base image")
	c.Flags().StringP("os-release-vars-path", "o", "", "the path to additional os-release vars for the generated image [Optional] ")
}
