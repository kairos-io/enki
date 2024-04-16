package utils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	containerdCompression "github.com/containerd/containerd/archive/compression"
	"github.com/google/go-containerregistry/pkg/name"
	container "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/kairos-io/enki/pkg/constants"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
	"github.com/spf13/viper"
)

// CreateSquashFS creates a squash file at destination from a source, with options
// TODO: Check validity of source maybe?
func CreateSquashFS(runner v1.Runner, logger v1.Logger, source string, destination string, options []string) error {
	// create args
	args := []string{source, destination}
	// append options passed to args in order to have the correct order
	// protect against options passed together in the same string , i.e. "-x add" instead of "-x", "add"
	var optionsExpanded []string
	for _, op := range options {
		optionsExpanded = append(optionsExpanded, strings.Split(op, " ")...)
	}
	args = append(args, optionsExpanded...)
	out, err := runner.Run("mksquashfs", args...)
	if err != nil {
		logger.Debugf("Error running squashfs creation, stdout: %s", out)
		logger.Errorf("Error while creating squashfs from %s to %s: %s", source, destination, err)
		return err
	}
	return nil
}

func GolangArchToArch(arch string) (string, error) {
	switch strings.ToLower(arch) {
	case constants.ArchAmd64:
		return constants.Archx86, nil
	case constants.ArchArm64:
		return constants.ArchArm64, nil
	default:
		return "", fmt.Errorf("invalid arch")
	}
}

// GetUkiCmdline returns the cmdline to be used for the kernel.
// The cmdline can be overridden by the user using the cmdline flag.
// For each cmdline passed, we generate a uki file with that cmdline
// extend-cmdline will just extend the default cmdline so we only create one efi file
// extra-cmdline will create a new efi file for each cmdline passed
func GetUkiCmdline() []string {
	// Extend only
	cmdlineExtend := viper.GetString("extend-cmdline")
	if cmdlineExtend != "" {
		return []string{constants.UkiCmdline + " " + constants.UkiCmdlineInstall + " " + cmdlineExtend}
	}

	// extra
	cmdlineOverride := viper.GetStringSlice("extra-cmdline")
	if len(cmdlineOverride) == 0 {
		// For no extra cmdline, we default to install mode
		return []string{constants.UkiCmdline + " " + constants.UkiCmdlineInstall}
	} else {
		// For extra cmdline, we default to install mode + extra cmdlines
		cmdline := []string{constants.UkiCmdline + " " + constants.UkiCmdlineInstall}
		for _, line := range cmdlineOverride {
			cmdline = append(cmdline, fmt.Sprintf("%s %s", constants.UkiCmdline, line))
		}
		return cmdline
	}
}

// GetUkiSingleCmdlines returns the single-efi-cmdline as passed by the user.
func GetUkiSingleCmdlines(logger v1.Logger) map[string]string {
	result := map[string]string{}
	// extra
	cmdlines := viper.GetStringSlice("single-efi-cmdline")
	for _, userValue := range cmdlines {
		userSplitValues := strings.SplitN(userValue, "=", 2)
		if len(userSplitValues) != 2 {
			logger.Warnf("bad value for single-efi-cmdline: %s", userValue)
			continue
		}
		result[userSplitValues[0]] = constants.UkiCmdline + " " + userSplitValues[1]
	}

	return result
}

// Tar takes a source and variable writers and walks 'source' writing each file
// found to the tar writer; the purpose for accepting multiple writers is to allow
// for multiple outputs (for example a file, or md5 hash)
func Tar(src string, writers ...io.Writer) error {
	// ensure the src actually exists before trying to tar it
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("Unable to tar files - %v", err.Error())
	}

	mw := io.MultiWriter(writers...)

	gzw := gzip.NewWriter(mw)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	// walk path
	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {

		// return on any error
		if err != nil {
			return err
		}

		// return on non-regular files (thanks to [kumo](https://medium.com/@komuw/just-like-you-did-fbdd7df829d3) for this suggested update)
		if !fi.Mode().IsRegular() {
			return nil
		}

		// create a new dir/file header
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		// update the name to correctly reflect the desired destination when untaring
		header.Name = strings.TrimPrefix(strings.Replace(file, src, "", -1), string(filepath.Separator))

		// write the header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// open files for taring
		f, err := os.Open(file)
		if err != nil {
			return err
		}

		// copy file data into tar writer
		if _, err := io.Copy(tw, f); err != nil {
			return err
		}

		// manually close here after each file operation; defering would cause each file close
		// to wait until all operations have completed.
		f.Close()

		return nil
	})
}

// CreateTar a imagetarball from a standard tarball
func CreateTar(log v1.Logger, srctar, dstimageTar, imagename, architecture, OS string) error {

	dstFile, err := os.Create(dstimageTar)
	if err != nil {
		return fmt.Errorf("Cannot create %s: %s", dstimageTar, err)
	}
	defer dstFile.Close()

	newRef, img, err := imageFromTar(imagename, architecture, OS, func() (io.ReadCloser, error) {
		f, err := os.Open(srctar)
		if err != nil {
			return nil, fmt.Errorf("cannot open %s: %s", srctar, err)
		}
		decompressed, err := containerdCompression.DecompressStream(f)
		if err != nil {
			return nil, fmt.Errorf("cannot open %s: %s", srctar, err)
		}

		return decompressed, nil
	})
	if err != nil {
		return err
	}

	// Lets try to load it into the docker daemon?
	// Code left here in case we want to use it in the future
	/*
		tag, err := name.NewTag(imagename)

		if err != nil {
			log.Warnf("Cannot create tag for %s: %s", imagename, err)
		}
		if err == nil {
			// Best effort only, just try and forget
			out, err := daemon.Write(tag, img)
			if err != nil {
				log.Warnf("Cannot write image %s to daemon: %s\noutput: %s", imagename, err, out)
			} else {
				log.Infof("Image %s written to daemon", tag.String())
			}
		}
	*/

	return tarball.Write(newRef, img, dstFile)

}

func imageFromTar(imagename, architecture, OS string, opener func() (io.ReadCloser, error)) (name.Reference, container.Image, error) {
	newRef, err := name.ParseReference(imagename)
	if err != nil {
		return nil, nil, err
	}

	layer, err := tarball.LayerFromOpener(opener)
	if err != nil {
		return nil, nil, err
	}

	baseImage := empty.Image
	cfg, err := baseImage.ConfigFile()
	if err != nil {
		return nil, nil, err
	}

	cfg.Architecture = architecture
	cfg.OS = OS

	baseImage, err = mutate.ConfigFile(baseImage, cfg)
	if err != nil {
		return nil, nil, err
	}
	img, err := mutate.Append(baseImage, mutate.Addendum{
		Layer: layer,
		History: container.History{
			CreatedBy: "Enki",
			Comment:   "Custom image",
			Created:   container.Time{Time: time.Now()},
		},
	})
	if err != nil {
		return nil, nil, err
	}

	return newRef, img, nil
}

func IsAmd64(arch string) bool {
	return arch == constants.ArchAmd64 || arch == constants.Archx86
}

func IsArm64(arch string) bool {
	return arch == constants.ArchArm64 || arch == constants.Archaarch64
}
