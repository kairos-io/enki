package action

import (
	"errors"
	"fmt"
	"os"
)

type DockerfileAction struct {
	RootFSPath        string
	BaseImageURI      string
	FrameworkImage    string
	OSReleaseVarsPath string
}

func (a *DockerfileAction) Run() (dockerfile string, err error) {
	if err := a.Validate(); err != nil {
		return "", err
	}

	dockerfile = ""
	dockerfile += a.baseImageSection()
	dockerfile += a.dnsSection()
	dockerfile += a.luetInstallSection("")
	dockerfile += a.installFrameworkSection()
	osReleaseSection, err := a.osReleaseSection(a.OSReleaseVarsPath)
	if err != nil {
		return "", err
	}
	dockerfile += osReleaseSection
	dockerfile += a.cleanupMachineIDSection()
	dockerfile += a.switchRootSection()
	dockerfile += a.enableServicesSection()
	dockerfile += a.osSpecificSection()

	return dockerfile, nil
}

func (a *DockerfileAction) baseImageSection() string {
	if a.BaseImageURI != "" {
		return fmt.Sprintf(`
FROM %s as base
FROM busybox as builder

COPY --from=base . /rootfs
`, a.BaseImageURI)
	}

	return fmt.Sprintf(`
FROM busybox as builder
RUN mkdir /rootfs
COPY %s /rootfs/.
`, a.RootFSPath)
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
COPY --from=quay.io/luet/base:%s /usr/bin/luet /usr/bin/luet
`, luetVersion)
}

func (a *DockerfileAction) osReleaseSection(filePath string) (string, error) {
	if filePath == "" {
		return "", nil
	}

	d, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`RUN cat <<EOF >> /rootfs/etc/os-release
%s
EOF
`, string(d)), nil
}

func (a *DockerfileAction) switchRootSection() string {
	return `
FROM scratch as rootfs

COPY --from=builder /rootfs/. /.

FROM rootfs
`
}

func (a *DockerfileAction) enableServicesSection() string {
	return `
RUN <<EOF
	if [ -f /sbin/openrc ]; then
		mkdir -p /etc/runlevels/default && \
			ln -sf /etc/init.d/cos-setup-boot /etc/runlevels/default/cos-setup-boot  && \
			ln -sf /etc/init.d/cos-setup-network /etc/runlevels/default/cos-setup-network  && \
			ln -sf /etc/init.d/cos-setup-reconcile /etc/runlevels/default/cos-setup-reconcile && \
			ln -sf /etc/init.d/kairos-agent /etc/runlevels/default/kairos-agent
	else
		# mask systemd-firstboot dont accidentally run block booting
		systemctl mask systemd-firstboot
		systemctl enable cos-setup-reconcile.timer && \
			systemctl enable cos-setup-fs.service && \
			systemctl enable cos-setup-boot.service && \
			systemctl enable cos-setup-network.service
	fi
EOF
`
}

// installFrameworkSection chooses the right framework image for the current
// base image and upacks it to the /rootfs directory
func (a *DockerfileAction) installFrameworkSection() string {
	return fmt.Sprintf(`
COPY --from=%s . /rootfs

# Avoid to accidentally push keys generated by package managers
RUN rm -rf /rootfs/etc/ssh/ssh_host_*
`, a.FrameworkImage)
}

func (a *DockerfileAction) osSpecificSection() string {
	return `
# Additional os specific things
`
}

// cleanupMachineIDSection removes machine-id. It will be generated on first boot.
func (a *DockerfileAction) cleanupMachineIDSection() string {
	return "\nRUN rm -rf /rootfs/etc/machine-id\n"
}

func (a *DockerfileAction) Validate() error {
	if a.RootFSPath != "" && a.BaseImageURI != "" ||
		a.RootFSPath == "" && a.BaseImageURI == "" {
		return errors.New("exactly one of 'rootfs-dir' and 'base-image-uri' should be defined")
	}

	if a.FrameworkImage == "" {
		return errors.New("'framework-image' should be defined")
	}

	return nil
}
