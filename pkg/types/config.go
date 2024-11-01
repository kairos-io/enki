package types

import (
	"fmt"

	cfg "github.com/kairos-io/kairos-agent/v2/pkg/config"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
)

type LiveISO struct {
	RootFS             []*v1.ImageSource `yaml:"rootfs,omitempty" mapstructure:"rootfs"`
	UEFI               []*v1.ImageSource `yaml:"uefi,omitempty" mapstructure:"uefi"`
	Image              []*v1.ImageSource `yaml:"image,omitempty" mapstructure:"image"`
	Label              string            `yaml:"label,omitempty" mapstructure:"label"`
	GrubEntry          string            `yaml:"grub-entry-name,omitempty" mapstructure:"grub-entry-name"`
	BootloaderInRootFs bool              `yaml:"bootloader-in-rootfs" mapstructure:"bootloader-in-rootfs"`
}

// BuildConfig represents the config we need for building isos, raw images, artifacts
type BuildConfig struct {
	Date bool `yaml:"date,omitempty" mapstructure:"date"`

	// 'inline' and 'squash' labels ensure config fields
	// are embedded from a yaml and map PoV
	cfg.Config `yaml:",inline" mapstructure:",squash"`
}

// Sanitize checks the consistency of the struct, returns error
// if unsolvable inconsistencies are found
func (i *LiveISO) Sanitize() error {
	for _, src := range i.RootFS {
		if src == nil {
			return fmt.Errorf("wrong name of source package for rootfs")
		}
	}
	for _, src := range i.UEFI {
		if src == nil {
			return fmt.Errorf("wrong name of source package for uefi")
		}
	}
	for _, src := range i.Image {
		if src == nil {
			return fmt.Errorf("wrong name of source package for image")
		}
	}

	return nil
}
