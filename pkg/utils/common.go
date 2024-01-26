package utils

import (
	"fmt"
	"github.com/kairos-io/enki/pkg/constants"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
	"github.com/spf13/viper"
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
// For each cmdline passed, we generate a uki file with that cmdline
func GetUkiCmdline() []string {
	cmdlineOverride := viper.GetStringSlice("cmdline")
	if len(cmdlineOverride) == 0 {
		return []string{constants.UkiCmdline}
	} else {
		cmdline := []string{constants.UkiCmdline}
		for _, line := range cmdlineOverride {
			cmdline = append(cmdline, fmt.Sprintf("%s %s", constants.UkiCmdline, line))
		}
		return cmdline
	}

}
