package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("build-uki", func() {
	var resultDir string
	var keysDir string
	var resultFile string
	var image string
	var err error
	var enki *Enki

	BeforeEach(func() {
		kairosVersion := "v2.5.0"
		resultDir, err = os.MkdirTemp("", "enki-build-uki-test-")
		Expect(err).ToNot(HaveOccurred())
		resultFile = filepath.Join(resultDir, fmt.Sprintf("kairos_%s.iso", kairosVersion))

		currentDir, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		keysDir = filepath.Join(currentDir, "assets", "keys")

		enki = NewEnki("enki-image", resultDir, keysDir)
		image = fmt.Sprintf("quay.io/kairos/fedora:38-core-amd64-generic-%s", kairosVersion)
	})

	AfterEach(func() {
		os.RemoveAll(resultDir)
		enki.Cleanup()
	})

	When("some dependency is missing", func() {
		BeforeEach(func() {
			enki = NewEnki("busybox", resultDir, keysDir)
		})

		It("returns an error about missing deps", func() {
			out, err := enki.Run("build-uki", image, "--output-dir", resultDir, "-k", "assets/keys", "--output-type", "iso")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(Or(
				MatchRegexp("executable file not found in \\$PATH"),
				MatchRegexp("no such file or directory"),
			))
		})
	})

	It("successfully builds a uki iso from a container image", func() {
		out, err := enki.Run("build-uki", image, "--output-dir", resultDir, "-k", keysDir, "--output-type", "iso")
		Expect(err).ToNot(HaveOccurred(), out)

		By("building the iso")
		_, err = os.Stat(resultFile)
		Expect(err).ToNot(HaveOccurred())

		By("booting the iso")
		// TODO: Move the uki test here from kairos?
	})
})
