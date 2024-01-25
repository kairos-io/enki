package utils

import (
	"compress/gzip"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/kairos-io/enki/pkg/constants"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
	"github.com/spf13/viper"
	"io"
	"path/filepath"
	"strings"
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
func GetUkiCmdline() string {
	cmdlineOverride := viper.GetString("cmdline")
	if cmdlineOverride == "" {
		return constants.UkiCmdline
	} else {
		return constants.UkiCmdline + " " + strings.Trim(cmdlineOverride, " ")
	}
}

func FindKairosVersion(fs v1.FS, sourceDir string) (string, error) {
	_, err := fs.Stat(filepath.Join(sourceDir, "etc", "os-release"))
	if err != nil {
		return "", fmt.Errorf("%s/etc/os-release file doesnt seem to exist: %w", sourceDir, err)
	}

	envMap, err := godotenv.Read(filepath.Join(sourceDir, "etc", "os-release"))
	if err != nil {
		return "", fmt.Errorf("reading os-release file: %w", err)
	}
	release := envMap["KAIROS_RELEASE"]

	if release == "" {
		return "", fmt.Errorf("KAIROS_RELEASE is empty")
	}

	return release, nil
}

// GzipFile compresses a file using gzip
func GzipFile(fs v1.FS, sourcePath, targetPath string) error {
	inputFile, err := fs.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("error opening initramfs file: %w", err)
	}
	defer inputFile.Close()

	outputFile, err := fs.Create(targetPath)
	if err != nil {
		return fmt.Errorf("error creating compressed initramfs file: %w", err)
	}
	defer outputFile.Close()

	gzipWriter := gzip.NewWriter(outputFile)
	defer gzipWriter.Close()

	if _, err = io.Copy(gzipWriter, inputFile); err != nil {
		return fmt.Errorf("error writing data to the compress initramfs file: %w", err)
	}

	return nil
}
