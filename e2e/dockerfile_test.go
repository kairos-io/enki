package e2e_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	//. "github.com/onsi/gomega"
)

var _ = Describe("dockefile generation", func() {
	Describe("adding kairos bits", func() {
		var imageName string
		var dockerfile string

		BeforeEach(func() {
			baseImage := "ubuntu:latest"
			frameworkImage := "quay.io/kairos/framework:master_ubuntu"

			// TODO: Generete os-release vars file
			dockerfile = generateDockerfile(baseImage, frameworkImage, "")
			imageName = buildDockerfile(dockerfile)
		})

		AfterEach(func() {
			os.RemoveAll(dockerfile)
			deleteImage(imageName)
		})

		It("adds overlay files", func() {
		})

		It("appends to /etc/os-release", func() {

		})
	})
})
