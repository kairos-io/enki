package action_test

import (
	"fmt"
	"os"
	"os/exec"

	. "github.com/kairos-io/enki/pkg/action"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("DockerfileAction", func() {
	var action *DockerfileAction

	When("both a rootfs dir and a base image URI are defined", func() {
		BeforeEach(func() {
			action = NewDockerfileAction("somedir", "quay.io/kairos/someimage")
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
				action = NewDockerfileAction(rootfsPath, "")
			})

			AfterEach(func() {
				os.RemoveAll(rootfsPath)
			})

			It("uses the provided rootfs as a base", func() {
				dockerfile, err := action.Run()
				Expect(err).ToNot(HaveOccurred())

				Expect(dockerfile).To(MatchRegexp("COPY . /rootfs/."))
			})
		})

		When("a base image uri is defined", func() {
			BeforeEach(func() {
				action = NewDockerfileAction("", "ubuntu:latest")
			})

			It("starts with the provided base image", func() {
				dockerfile, err := action.Run()
				Expect(err).ToNot(HaveOccurred())

				Expect(dockerfile).To(MatchRegexp("FROM ubuntu:latest as builder"))
			})
		})
	})

	Describe("adding kairos bits", func() {
		When("rootfs dir is defined", func() {
			var rootfsPath string

			BeforeEach(func() {
				rootfsPath = prepareRootfsFromImage("ubuntu:latest")
				action = NewDockerfileAction(rootfsPath, "")
			})

			AfterEach(func() {
				cleanupDir(rootfsPath)
			})

			It("adds Kairos bits", func() {
				dockerfile, err := action.Run()
				Expect(err).ToNot(HaveOccurred())
				Expect(dockerfile).To(MatchRegexp("luet install k3s")) // TODO: Change this to actual bits
			})
		})

		When("base image URI is defined", func() {
			BeforeEach(func() {
				action = NewDockerfileAction("", "ubuntu:latest")
			})

			It("adds Kairos bits", func() {
				dockerfile, err := action.Run()
				Expect(err).ToNot(HaveOccurred())
				Expect(dockerfile).To(MatchRegexp("luet install k3s")) // TODO: Change this to actual bits
			})
		})
	})
})

func prepareEmptyRootfs() string {
	dir, err := os.MkdirTemp("", "kairos-temp")
	Expect(err).ToNot(HaveOccurred())

	return dir
}
func prepareRootfsFromImage(imageURI string) string {
	dir, err := os.MkdirTemp("", "kairos-temp")
	Expect(err).ToNot(HaveOccurred())

	cmd := exec.Command("/bin/sh", "-c",
		fmt.Sprintf("docker run -v %s:/work quay.io/luet/base util unpack %s /work", dir, imageURI))
	out, err := cmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(out))

	return dir
}
