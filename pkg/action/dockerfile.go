package action

import (
	"errors"
	"fmt"
)

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
	if err := a.Validate(); err != nil {
		return "", err
	}

	dockerfile = ""
	dockerfile += a.baseImageSection()
	dockerfile += a.dnsSection()
	dockerfile += a.luetInstallSection("")
	dockerfile += a.footerSection()
	dockerfile += a.osSpecificSection()

	return dockerfile, nil
}

func (a *DockerfileAction) baseImageSection() string {
	result := ""
	if a.baseImageURI != "" {
		return fmt.Sprintf(`
FROM %s as base
FROM busybox as builder

COPY --from=base . /rootfs
`, a.baseImageURI)
	}

	result = fmt.Sprintf(`
FROM busybox as builder
RUN mkdir /rootfs
COPY %s /rootfs/.
`, a.rootFSPath)

	return result
}

func (a *DockerfileAction) dnsSection() string {
	return `
RUN echo "nameserver 8.8.8.8" > /rootfs/etc/resolv.conf
RUN cat /rootfs/etc/resolv.conf
`
}

func (a *DockerfileAction) luetInstallSection(luetVersion string) string {
	if luetVersion == "" {
		luetVersion = "latest"
	}

	return fmt.Sprintf(`
COPY --from=quay.io/luet/base:%s /usr/bin/luet
`, luetVersion)
}

func (a *DockerfileAction) footerSection() string {
	return `
FROM scratch as rootfs

COPY --from=builder /rootfs/ .
`
}

func (a *DockerfileAction) osSpecificSection() string {
	return `
FROM rootfs
# Additional os specific things
`
}

func (a *DockerfileAction) Validate() error {
	if a.rootFSPath != "" && a.baseImageURI != "" ||
		a.rootFSPath == "" && a.baseImageURI == "" {
		return errors.New("exactly one of 'rootfs-dir' and 'base-image-uri' should be defined")
	}

	return nil
}
