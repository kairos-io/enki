package action_test

import (
	"os"

	. "github.com/kairos-io/enki/pkg/action"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DockerfileAction", func() {
	var action *DockerfileAction

	When("both a rootfs dir and a base image URI are defined", func() {
		BeforeEach(func() {
			action = &DockerfileAction{
				RootFSPath:     "somedir",
				BaseImageURI:   "quay.io/kairos/someimage",
				FrameworkImage: "quay.io/kairos/framework:master_ubuntu",
			}
		})

		It("returns an error", func() {
			_, err := action.Run()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("defining the base image", func() {
		When("rootfs dir is defined", func() {
			var rootfsPath string

			BeforeEach(func() {
				rootfsPath = prepareEmptyRootfs()
				action = &DockerfileAction{
					RootFSPath:     rootfsPath,
					FrameworkImage: "quay.io/kairos/framework:master_ubuntu",
				}
			})

			AfterEach(func() {
				os.RemoveAll(rootfsPath)
			})

			It("uses the provided rootfs as a base", func() {
				dockerfile, err := action.Run()
				Expect(err).ToNot(HaveOccurred())

				Expect(dockerfile).To(MatchRegexp("COPY --from=builder /rootfs/ ."))
			})
		})

		When("a base image uri is defined", func() {
			BeforeEach(func() {
				action = &DockerfileAction{
					BaseImageURI:   "ubuntu:latest",
					FrameworkImage: "quay.io/kairos/framework:master_ubuntu",
				}
			})

			It("starts with the provided base image", func() {
				dockerfile, err := action.Run()
				Expect(err).ToNot(HaveOccurred())

				Expect(dockerfile).To(MatchRegexp("FROM ubuntu:latest as base"))
			})
		})
	})

	Describe("adding kairos bits", func() {
		var osReleaseVarsFile string

		BeforeEach(func() {
			osReleaseVarsFile = createOsReleaseVarsFile("MYVAR=MYVAL")

			action = &DockerfileAction{
				FrameworkImage:    "quay.io/kairos/framework:master_ubuntu",
				OSReleaseVarsPath: osReleaseVarsFile,
			}
		})

		AfterEach(func() {
			Expect(os.RemoveAll(osReleaseVarsFile)).ToNot(HaveOccurred())
		})

		When("rootfs dir is defined", func() {
			var rootfsPath string

			BeforeEach(func() {
				rootfsPath = prepareRootfsFromImage("ubuntu:latest")
				action.RootFSPath = rootfsPath
			})

			AfterEach(func() {
				cleanupDir(rootfsPath)
			})

			It("adds Kairos bits", func() {
				checkForKairosBits(action)
			})
		})

		When("base image URI is defined", func() {
			BeforeEach(func() {
				action.BaseImageURI = "ubuntu:latest"
			})

			It("adds Kairos bits", func() {
				checkForKairosBits(action)
			})
		})
	})
})

func checkForKairosBits(action *DockerfileAction) {
	dockerfile, err := action.Run()
	Expect(err).ToNot(HaveOccurred())

	dockerfileMustHaveLuet(dockerfile)
	dockerfileMustInstallFramework(dockerfile)
	dockerfileMustIncludeAdditionalOSRelaseVars(dockerfile)
}

func dockerfileMustHaveLuet(d string) {
	By("checking for installation of luet")
	Expect(d).To(MatchRegexp("quay.io/luet/base.* /usr/bin/luet"))
}

func dockerfileMustInstallFramework(d string) {
	By("checking installation of framework bits")
	Expect(d).To(MatchRegexp("COPY --from=quay.io/kairos/framework"))
}

func dockerfileMustIncludeAdditionalOSRelaseVars(d string) {
	By("checking /etc/os-release vars")
	Expect(d).To(MatchRegexp("/etc/os-release\nMYVAR=MYVAL"))
}

func createOsReleaseVarsFile(content string) string {
	file, err := os.CreateTemp("", "kairos-os-release-")
	Expect(err).ToNot(HaveOccurred())
	_, err = file.Write([]byte("MYVAR=MYVAL"))
	Expect(err).ToNot(HaveOccurred())

	return file.Name()
}
