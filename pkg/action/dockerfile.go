package action

import "errors"

// ConverterAction is the action that converts a non-kairos image to a Kairos one.
// The conversion happens in a best-effort manner. It's not guaranteed that
// any distribution will successfully be converted to a Kairos flavor. See
// the Kairos releases for known-to-work flavors.
// The "input" of this action is a directory where the rootfs is extracted.
// [TBD] The output is the same directory updated to be a Kairos image
type DockerfileAction struct {
	rootFSPath   string
	baseImageURI string
}

func NewDockerfileAction(rootfsPath, baseImageURI string) *DockerfileAction {
	return &DockerfileAction{
		rootFSPath:   rootfsPath,
		baseImageURI: baseImageURI,
	}
}

func (a *DockerfileAction) Run() (dockerfile string, err error) {
	if a.rootFSPath != "" && a.baseImageURI != "" {
		return "", errors.New("only one of 'rootfs-dir' and 'base-image-uri' should be defined")
	}

	return "", nil
}
