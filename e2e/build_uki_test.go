package e2e_test

import (
	"os"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("build-uki", func() {
	var resultDir string
	var resultFile string
	var image string
	var err error
	var enki *Enki

	BeforeEach(func() {
		resultDir, err = os.MkdirTemp("", "enki-build-uki-test-")
		Expect(err).ToNot(HaveOccurred())
		resultFile = path.Join(resultDir, "result.uki")

		enki = NewEnki("enki-image", resultDir)
		image = "busybox"
	})

	AfterEach(func() {
		os.RemoveAll(resultDir)
		enki.Cleanup()
	})

	When("some dependency is missing", func() {
		BeforeEach(func() {
			enki = NewEnki("busybox", resultDir)
		})

		It("returns an error about missing deps", func() {
			out, err := enki.Run("build-uki", image, resultFile)
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(Or(
				MatchRegexp("executable file not found in \\$PATH"),
				MatchRegexp("no such file or directory"),
			))
		})
	})

	It("successfully builds an UKI from a Docker image", func() {
		out, err := enki.Run("build-uki", image, resultFile)
		Expect(err).ToNot(HaveOccurred(), out)

		_, err = os.Stat(resultFile)
		Expect(err).ToNot(HaveOccurred())
	})
})
