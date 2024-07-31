package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("build-uki", Label("build-uki", "e2e"), func() {
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

	Describe("single-efi-cmdline", func() {
		BeforeEach(func() {
			By("building the iso with single-efi-cmdline flags set")
			buildISO(enki, image, keysDir, resultDir, resultFile,
				"--single-efi-cmdline", "My Entry: someoption=somevalue",
				"--single-efi-cmdline", "My Other Entry: someoption2=somevalue2")
		})

		It("creates additional .efi and .conf files", func() {
			By("checking if the default value for secure-boot-enroll is set")
			content := listEfiFiles(enki, resultFile)
			Expect(string(content)).To(MatchRegexp("My_Entry.efi"))
			Expect(string(content)).To(MatchRegexp("My_Other_Entry.efi"))

			content = listConfFiles(enki, resultFile)
			Expect(string(content)).To(MatchRegexp("My_Entry.conf"))
			Expect(string(content)).To(MatchRegexp("My_Other_Entry.conf"))
		})
	})

	Describe("secure-boot-enroll setting in loader.conf", func() {
		When("secure-boot-enroll is not set", func() {
			BeforeEach(func() {
				By("building the iso with secure-boot-enroll not set")
				buildISO(enki, image, keysDir, resultDir, resultFile)
			})

			It("sets the secure-boot-enroll correctly", func() {
				By("checking if the default value for secure-boot-enroll is set")
				content := readLoaderConf(enki, resultFile)
				Expect(string(content)).To(MatchRegexp("secure-boot-enroll if-safe"))
			})
		})

		When("secure-boot-enroll is set", func() {
			BeforeEach(func() {
				By("building the iso with secure-boot-enroll set to manual")
				buildISO(enki, image, keysDir, resultDir, resultFile, "--secure-boot-enroll", "manual")
			})

			It("sets the secure-boot-enroll correctly", func() {
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
	return runCommandInIso(enki, isoFile, "cat /tmp/efi/loader/loader.conf")
}

func listEfiFiles(enki *Enki, isoFile string) string {
	return runCommandInIso(enki, isoFile, "ls /tmp/efi/EFI/kairos")
}

func listConfFiles(enki *Enki, isoFile string) string {
	return runCommandInIso(enki, isoFile, "ls /tmp/efi/loader/entries")
}

func runCommandInIso(enki *Enki, isoFile, command string) string {
	By("running command: " + command)
	out, err := enki.ContainerRun("/bin/bash", "-c",
		fmt.Sprintf(`#!/bin/bash
set -e
mkdir -p /tmp/iso /tmp/efi
mount -v -o loop %[1]s /tmp/iso 2>&1 > /dev/null
mount -v -o loop /tmp/iso/efiboot.img /tmp/efi 2>&1 > /dev/null
%[2]s
umount /tmp/efi 2>&1 > /dev/null
umount /tmp/iso 2>&1 > /dev/null
`, isoFile, command))
	Expect(err).ToNot(HaveOccurred(), out)

	return out
}
