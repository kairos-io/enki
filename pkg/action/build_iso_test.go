/*
   Copyright © 2022 SUSE LLC

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package action_test

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/kairos-io/enki/pkg/action"
	"github.com/kairos-io/enki/pkg/config"
	"github.com/kairos-io/enki/pkg/constants"
	"github.com/kairos-io/enki/pkg/types"
	"github.com/kairos-io/enki/pkg/utils"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
	v1mock "github.com/kairos-io/kairos-agent/v2/tests/mocks"
	sdkTypes "github.com/kairos-io/kairos-sdk/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"
)

var _ = Describe("BuildISOAction", func() {
	var cfg *types.BuildConfig
	var runner *v1mock.FakeRunner
	var fs vfs.FS
	var logger sdkTypes.KairosLogger
	var syscall *v1mock.FakeSyscall
	var client *v1mock.FakeHTTPClient
	var cloudInit *v1mock.FakeCloudInitRunner
	var cleanup func()
	var memLog *bytes.Buffer
	var imageExtractor *v1mock.FakeImageExtractor
	BeforeEach(func() {
		runner = v1mock.NewFakeRunner()
		syscall = &v1mock.FakeSyscall{}
		client = &v1mock.FakeHTTPClient{}
		memLog = &bytes.Buffer{}
		logger = sdkTypes.NewBufferLogger(memLog)
		logger.SetLevel("debug")
		cloudInit = &v1mock.FakeCloudInitRunner{}
		fs, cleanup, _ = vfst.NewTestFS(map[string]interface{}{})
		imageExtractor = v1mock.NewFakeImageExtractor(logger)

		cfg = config.NewBuildConfig(
			config.WithFs(fs),
			config.WithRunner(runner),
			config.WithLogger(logger),
			config.WithSyscall(syscall),
			config.WithClient(client),
			config.WithCloudInitRunner(cloudInit),
			config.WithImageExtractor(imageExtractor),
		)
	})
	AfterEach(func() {
		cleanup()
	})
	Describe("Build ISO", Label("iso"), func() {
		var iso *types.LiveISO
		BeforeEach(func() {
			iso = config.NewISO()

			tmpDir, err := utils.TempDir(fs, "", "test")
			Expect(err).ShouldNot(HaveOccurred())

			cfg.Date = false
			cfg.OutDir = tmpDir

			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				switch cmd {
				case "xorriso":
					err := fs.WriteFile(filepath.Join(tmpDir, "elemental.iso"), []byte("profound thoughts"), constants.FilePerm)
					return []byte{}, err
				default:
					return []byte{}, nil
				}
			}
		})
		It("Successfully builds an ISO from a Docker image", func() {
			rootSrc, _ := v1.NewSrcFromURI("oci:image:version")
			iso.RootFS = []*v1.ImageSource{rootSrc}
			uefiSrc, _ := v1.NewSrcFromURI("oci:image:version")
			iso.UEFI = []*v1.ImageSource{uefiSrc}
			imageSrc, _ := v1.NewSrcFromURI("oci:image:version")
			iso.Image = []*v1.ImageSource{imageSrc}

			// Create kernel and vmlinuz
			// Thanks to the testfs stuff in utils.TempDir we know what the temp fs is gonna be as
			// its predictable
			bootDir := filepath.Join("/tmp/enki-iso/rootfs", "boot")
			err := utils.MkdirAll(fs, bootDir, constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create(filepath.Join(bootDir, "vmlinuz"))
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create(filepath.Join(bootDir, "initrd"))
			Expect(err).ShouldNot(HaveOccurred())
			// Create shim and efi fake files
			err = utils.MkdirAll(fs, filepath.Join(bootDir, "efi", "EFI", "fedora"), constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create(filepath.Join(bootDir, "efi", "EFI", "fedora", "shim.efi"))
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create(filepath.Join(bootDir, "efi", "EFI", "fedora", "grubx64.efi"))
			Expect(err).ShouldNot(HaveOccurred())

			buildISO := action.NewBuildISOAction(cfg, iso)
			err = buildISO.ISORun()

			Expect(err).ShouldNot(HaveOccurred())
		})
		It("Fails if kernel or initrd is not found in rootfs", func() {
			rootSrc, _ := v1.NewSrcFromURI("oci:image:version")
			iso.RootFS = []*v1.ImageSource{rootSrc}
			uefiSrc, _ := v1.NewSrcFromURI("oci:image:version")
			iso.UEFI = []*v1.ImageSource{uefiSrc}
			imageSrc, _ := v1.NewSrcFromURI("oci:image:version")
			iso.Image = []*v1.ImageSource{imageSrc}

			By("fails without kernel")
			buildISO := action.NewBuildISOAction(cfg, iso)
			err := buildISO.ISORun()
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("No file found with prefixes"))
			Expect(err.Error()).To(ContainSubstring("uImage Image zImage vmlinuz image"))

			bootDir := filepath.Join("/tmp/enki-iso/rootfs", "boot")
			err = utils.MkdirAll(fs, bootDir, constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create(filepath.Join(bootDir, "vmlinuz"))
			Expect(err).ShouldNot(HaveOccurred())

			By("fails without initrd")
			buildISO = action.NewBuildISOAction(cfg, iso)
			err = buildISO.ISORun()
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("No file found with prefixes"))
			Expect(err.Error()).To(ContainSubstring("initrd initramfs"))
		})
		It("Fails installing image sources", func() {
			rootSrc, _ := v1.NewSrcFromURI("oci:image:version")
			iso.RootFS = []*v1.ImageSource{rootSrc}
			uefiSrc, _ := v1.NewSrcFromURI("oci:image:version")
			iso.UEFI = []*v1.ImageSource{uefiSrc}
			imageSrc, _ := v1.NewSrcFromURI("oci:image:version")
			iso.Image = []*v1.ImageSource{imageSrc}

			imageExtractor.SideEffect = func(imageRef, destination, platformRef string) error {
				return fmt.Errorf("uh oh")
			}

			buildISO := action.NewBuildISOAction(cfg, iso)
			err := buildISO.ISORun()
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("uh oh"))
		})
		It("Fails on ISO filesystem creation", func() {
			rootSrc, _ := v1.NewSrcFromURI("oci:image:version")
			iso.RootFS = []*v1.ImageSource{rootSrc}
			uefiSrc, _ := v1.NewSrcFromURI("oci:image:version")
			iso.UEFI = []*v1.ImageSource{uefiSrc}
			imageSrc, _ := v1.NewSrcFromURI("oci:image:version")
			iso.Image = []*v1.ImageSource{imageSrc}

			bootDir := filepath.Join("/tmp/enki-iso/rootfs", "boot")
			err := utils.MkdirAll(fs, bootDir, constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create(filepath.Join(bootDir, "vmlinuz"))
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create(filepath.Join(bootDir, "initrd"))
			Expect(err).ShouldNot(HaveOccurred())
			// Create shim and efi fake files
			err = utils.MkdirAll(fs, filepath.Join(bootDir, "efi", "EFI", "fedora"), constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create(filepath.Join(bootDir, "efi", "EFI", "fedora", "shim.efi"))
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create(filepath.Join(bootDir, "efi", "EFI", "fedora", "grubx64.efi"))
			Expect(err).ShouldNot(HaveOccurred())

			runner.SideEffect = func(command string, args ...string) ([]byte, error) {
				if command == "xorriso" {
					return []byte{}, errors.New("Burn ISO error")
				}
				return []byte{}, nil
			}

			buildISO := action.NewBuildISOAction(cfg, iso)
			err = buildISO.ISORun()

			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Burn ISO error"))
		})
	})
})
