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
		Expect(os.MkdirAll(keysDir, 0755)).ToNot(HaveOccurred())

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
			out, err := enki.Run("build-uki", image, "--output-dir", resultDir, "-k", keysDir, "--output-type", "iso")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(Or(
				MatchRegexp("executable file not found in \\$PATH"),
				MatchRegexp("no such file or directory"),
			))
		})
	})

	Describe("building an iso", func() {
		When("secure-boot-enroll is not set", func() {
			It("successfully builds a uki iso from a container image", func() {
				By("building the iso with secure-boot-enroll not set")
				buildISO(enki, image, keysDir, resultDir, resultFile)
				By("checking if the default value for secure-boot-enroll is set")
				content := readLoaderConf(enki, resultFile)
				Expect(string(content)).To(MatchRegexp("secure-boot-enroll if-safe"))
			})
		})

		When("secure-boot-enroll is set", func() {
			It("successfully builds a uki iso from a container image", func() {
				By("building the iso with secure-boot-enroll set to manual")
				buildISO(enki, image, keysDir, resultDir, resultFile, "--secure-boot-enroll", "manual")
				By("checking if the user value for secure-boot-enroll is set")
				content := readLoaderConf(enki, resultFile)
				Expect(string(content)).To(MatchRegexp("secure-boot-enroll manual"))
			})
		})
	})
})

func buildISO(enki *Enki, image, keysDir, resultDir, resultFile string, additionalArgs ...string) {
	out, err := enki.Run(append([]string{"build-uki", image, "--output-dir", resultDir,
		"-k", keysDir, "--output-type", "iso"}, additionalArgs...)...)
	Expect(err).ToNot(HaveOccurred(), out)

	By("building the iso")
	_, err = os.Stat(resultFile)
	Expect(err).ToNot(HaveOccurred())
}

func readLoaderConf(enki *Enki, isoFile string) string {
	By("checking the loader.conf file")
	out, err := enki.ContainerRun("/bin/bash", "-c",
		fmt.Sprintf(`#!/bin/bash
set -e
mkdir -p /tmp/iso /tmp/efi
mount -v -o loop %[1]s /tmp/iso 2>&1 > /dev/null
mount -v -o loop /tmp/iso/efiboot.img /tmp/efi 2>&1 > /dev/null
cat /tmp/efi/loader/loader.conf`, isoFile))
	Expect(err).ToNot(HaveOccurred(), out)

	return out
}
