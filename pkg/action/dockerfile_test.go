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
			action = NewDockerfileAction("somedir", "quay.io/kairos/someimage", "quay.io/kairos/framework:master_ubuntu")
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
				action = NewDockerfileAction(rootfsPath, "", "quay.io/kairos/framework:master_ubuntu")
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
				action = NewDockerfileAction("", "ubuntu:latest", "quay.io/kairos/framework:master_ubuntu")
			})

			It("starts with the provided base image", func() {
				dockerfile, err := action.Run()
				Expect(err).ToNot(HaveOccurred())

				Expect(dockerfile).To(MatchRegexp("FROM ubuntu:latest as base"))
			})
		})
	})

	Describe("adding kairos bits", func() {
		When("rootfs dir is defined", func() {
			var rootfsPath string

			BeforeEach(func() {
				rootfsPath = prepareRootfsFromImage("ubuntu:latest")
				action = NewDockerfileAction(rootfsPath, "", "quay.io/kairos/framework:master_ubuntu")
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
				action = NewDockerfileAction("", "ubuntu:latest", "quay.io/kairos/framework:master_ubuntu")
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
}

func dockerfileMustHaveLuet(d string) {
	By("checking for installation of luet")
	Expect(d).To(MatchRegexp("quay.io/luet/base.* /usr/bin/luet"))
}

func dockerfileMustInstallFramework(d string) {
	By("checking installation of framework bits")
	Expect(d).To(MatchRegexp("COPY --from=quay.io/kairos/framework"))
}
