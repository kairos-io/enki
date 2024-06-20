package action

import (
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kairos-io/enki/pkg/constants"
	"github.com/klauspost/compress/zstd"
	"github.com/sanity-io/litter"
	"github.com/spf13/viper"
	"github.com/u-root/u-root/pkg/cpio"
	"golang.org/x/exp/maps"

	"github.com/kairos-io/enki/pkg/types"
	"github.com/kairos-io/enki/pkg/utils"
	"github.com/kairos-io/kairos-agent/v2/pkg/elemental"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"

	"github.com/itxaka/go-ukify/pkg/uki"
)

type BuildUKIAction struct {
	img           *v1.ImageSource
	e             *elemental.Elemental
	outputDir     string
	keysDirectory string
	logger        v1.Logger
	outputType    string
	version       string
	arch          string
	name          string
}

func NewBuildUKIAction(cfg *types.BuildConfig, img *v1.ImageSource, outputDir, keysDirectory, outputType string) *BuildUKIAction {
	b := &BuildUKIAction{
		logger:        cfg.Logger,
		img:           img,
		e:             elemental.NewElemental(&cfg.Config),
		outputDir:     outputDir,
		keysDirectory: keysDirectory,
		outputType:    outputType,
		arch:          cfg.Arch,
		name:          cfg.Name,
	}
	b.logger.Debugf("BuildUKIAction: %+v", litter.Sdump(b))
	return b
}

func (b *BuildUKIAction) Run() error {
	err := b.checkDeps()
	if err != nil {
		return err
	}
	// artifactsTempDir Is where we copy the kernel and initramfs files
	// So only artifacts that are needed to build the efi, so we dont pollute the sourceDir
	artifactsTempDir, err := os.MkdirTemp("", "enki-build-uki-artifacts-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(artifactsTempDir)
	b.logger.Info("Extracting image to a temporary directory")
	// Source dir is the directory where we extract the image
	// It should only contain the image files and whatever changes we add or remove like creating dir or removing leftover
	// lets not pollute it
	sourceDir, err := b.extractImage()
	if err != nil {
		return err
	}
	defer os.RemoveAll(sourceDir)

	if viper.GetString("overlay-rootfs") != "" {
		b.logger.Infof("Adding files from %s to rootfs", viper.GetString("overlay-rootfs"))
		overlay, err := v1.NewSrcFromURI(fmt.Sprintf("dir:%s", viper.GetString("overlay-rootfs")))
		if err != nil {
			b.logger.Errorf("error creating overlay image: %s", err)
			return err
		}
		_, err = b.e.DumpSource(sourceDir, overlay)

		if err != nil {
			b.logger.Errorf("error copying overlay image: %s", err)
			return err
		}
	}

	// Store the version so we only need to check it once
	kairosVersion, err := findKairosVersion(sourceDir)
	if err != nil {
		return err
	}
	b.version = kairosVersion

	b.logger.Info("Creating additional directories in the rootfs")
	if err := b.setupDirectoriesAndFiles(sourceDir); err != nil {
		return err
	}

	b.logger.Info("Copying kernel")
	if err := b.copyKernel(sourceDir, artifactsTempDir); err != nil {
		return err
	}

	b.logger.Info("Cleaning up the source directory")
	b.cleanSource(sourceDir)

	b.logger.Info("Creating an initramfs file")
	if err := b.createInitramfs(sourceDir, artifactsTempDir); err != nil {
		return err
	}

	entries := append(utils.GetUkiCmdline(), utils.GetUkiSingleCmdlines(b.logger)...)
	for _, entry := range entries {
		b.logger.Info(fmt.Sprintf("Running ukify for cmdline: %s: %s", entry.Title, entry.Cmdline))

		b.logger.Infof("Generating: " + entry.FileName + ".efi")

		// New ukifier !!
		// Create Builder instance
		stub, err := b.getEfiStub()
		if err != nil {
			return err
		}
		// Get systemd-boot info (we can sign it at the same time)
		var systemdBoot string
		var outputSystemdBootEfi string
		if utils.IsAmd64(b.arch) {
			systemdBoot = constants.UkiSystemdBootx86
			outputSystemdBootEfi = constants.EfiFallbackNamex86
		} else if utils.IsArm64(b.arch) {
			systemdBoot = constants.UkiSystemdBootArm
			outputSystemdBootEfi = constants.EfiFallbackNameArm
		} else {
			return fmt.Errorf("unsupported arch: %s", b.arch)
		}

		builder := &uki.Builder{
			Arch:          b.arch,
			Version:       b.version,
			SdStubPath:    stub,
			KernelPath:    filepath.Join(artifactsTempDir, "vmlinuz"),
			InitrdPath:    filepath.Join(artifactsTempDir, "initrd"),
			Cmdline:       entry.Cmdline,
			OsRelease:     filepath.Join(sourceDir, "etc/os-release"),
			OutUKIPath:    entry.FileName + ".efi",
			PCRKey:        filepath.Join(b.keysDirectory, "tpm2-pcr-private.pem"),
			SBKey:         filepath.Join(b.keysDirectory, "db.key"),
			SBCert:        filepath.Join(b.keysDirectory, "db.pem"),
			SdBootPath:    systemdBoot,
			OutSdBootPath: outputSystemdBootEfi,
			LogLevel:      b.logger.GetLevel().String(),
		}

		if err := os.Chdir(sourceDir); err != nil {
			return fmt.Errorf("changing to %s directory: %w", sourceDir, err)
		}

		if err := builder.Build(); err != nil {
			return err
		}

		b.logger.Info("Creating kairos and loader conf files")
		if err := b.createConfFiles(sourceDir, entry.Cmdline, entry.Title, entry.FileName); err != nil {
			return err
		}
	}

	err = b.createSystemdConf(sourceDir)
	if err != nil {
		return err
	}

	switch b.outputType {
	case string(constants.IsoOutput):
		err = b.createISO(sourceDir)
		b.logger.Infof("Done building %s at: %s", b.outputType, b.outputDir)
	case string(constants.ContainerOutput):
		// First create the files
		err = b.createArtifact(sourceDir)
		if err != nil {
			return err
		}
		// Then build the image

		err = b.createContainer(b.outputDir, kairosVersion)
		if err != nil {
			return err
		}
		//Then remove the output dir files as we dont need them, the container has been loaded
		err = b.removeUkiFiles()
		if err != nil {
			return err
		}
	case string(constants.DefaultOutput):
		err = b.createArtifact(sourceDir)
		if err != nil {
			return err
		}
		b.logger.Infof("Done building %s at: %s", b.outputType, b.outputDir)
	}

	return err
}

// createSystemdConf creates the generic conf that systemd-boot uses
func (b *BuildUKIAction) createSystemdConf(sourceDir string) error {
	var finalEfiConf string
	entry := viper.GetString("default-entry")
	if entry != "" {
		if !strings.HasSuffix(entry, ".conf") {
			finalEfiConf = strings.TrimSuffix(entry, " ") + ".conf"
		} else {
			finalEfiConf = entry
		}

	} else {
		// Get the generic efi file that we produce from the default cmdline
		// This is the one name that has nothing added, just the version
		finalEfiConf = utils.NameFromCmdline(constants.ArtifactBaseName, constants.UkiCmdline+" "+constants.UkiCmdlineInstall) + ".conf"
	}

	secureBootEnroll := viper.GetString("secure-boot-enroll")
	// Set that as default selection for booting
	data := fmt.Sprintf("default %s\ntimeout 5\nconsole-mode max\neditor no\nsecure-boot-enroll %s\n", finalEfiConf, secureBootEnroll)
	err := os.WriteFile(filepath.Join(sourceDir, "loader.conf"), []byte(data), os.ModePerm)
	if err != nil {
		return fmt.Errorf("creating the loader.conf file: %s", err)
	}
	return nil
}

func (b *BuildUKIAction) extractImage() (string, error) {
	// TODO: if b.img is a dir, we should not copy or rsync anything and just use that dir as source?
	tmpDir, err := os.MkdirTemp("", "enki-build-uki-")
	if err != nil {
		return tmpDir, err
	}

	// By default MkdirTemp creates the dir with 0700 permissions, this results in an unusable system because all other users cannot access the sockets.
	err = os.Chmod(tmpDir, 0755)
	if err != nil {
		return tmpDir, err
	}

	_, err = b.e.DumpSource(tmpDir, b.img)

	return tmpDir, err
}

func (b *BuildUKIAction) checkDeps() error {
	neededBinaries := []string{
		"dd",
		"mkfs.msdos",
		"mmd",
		"mcopy",
		"xorriso",
	}

	for _, b := range neededBinaries {
		_, err := exec.LookPath(b)
		if err != nil {
			return err
		}
	}

	neededFiles, err := b.getEfiNeededFiles()
	if err != nil {
		return err
	}

	for _, b := range neededFiles {
		_, err := os.Stat(b)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *BuildUKIAction) setupDirectoriesAndFiles(tmpDir string) error {
	if err := os.Symlink("/usr/bin/immucore", filepath.Join(tmpDir, "init")); err != nil {
		return fmt.Errorf("error creating symlink: %w", err)
	}

	// able to mount oem under here if found
	if err := os.MkdirAll(filepath.Join(tmpDir, "oem"), os.ModeDir); err != nil {
		return fmt.Errorf("error creating /oem dir: %w", err)
	}

	// mount the esp under here if found
	if err := os.MkdirAll(filepath.Join(tmpDir, "efi"), os.ModeDir); err != nil {
		return fmt.Errorf("error creating /oem dir: %w", err)
	}

	// for install/upgrade they copy stuff there
	if err := os.MkdirAll(filepath.Join(tmpDir, "usr/local/cloud-config"), os.ModeDir); err != nil {
		return fmt.Errorf("error creating /oem dir: %w", err)
	}

	return nil
}

// createInitramfs creates a compressed initramfs file (cpio format, gzipped).
// The resulting file is named "initrd" and is saved inthe sourceDir.
func (b *BuildUKIAction) createInitramfs(sourceDir, artifactsTempDir string) error {
	format := "newc"
	archiver, err := cpio.Format(format)
	if err != nil {
		return fmt.Errorf("format %q not supported: %w", format, err)
	}

	cpioFileName := filepath.Join(artifactsTempDir, "initramfs.cpio")
	cpioFile, err := os.Create(cpioFileName)
	if err != nil {
		return fmt.Errorf("creating cpio file: %w", err)
	}
	defer cpioFile.Close()

	rw := archiver.Writer(cpioFile)
	cr := cpio.NewRecorder()

	// List of directories to exclude
	excludeDirs := map[string]bool{
		"sys":  true,
		"run":  true,
		"dev":  true,
		"tmp":  true,
		"proc": true,
	}

	if err = os.Chdir(sourceDir); err != nil {
		return fmt.Errorf("changing to %s directory: %w", sourceDir, err)
	}

	// Walk through the source directory and add files to the cpio archive
	err = filepath.Walk(".", func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if the current directory should be excluded
		if fileInfo.IsDir() && excludeDirs[filePath] {
			return filepath.SkipDir
		}

		if strings.Contains(filePath, "initramfs.cpio") {
			return nil
		}

		rec, err := cr.GetRecord(filePath)
		if err != nil {
			return fmt.Errorf("getting record of %q failed: %w", filePath, err)
		}

		// In case the record contains the sourceDir we want to remove it as its not part of the cpio initramfs
		// All files should have the proper path for the initramfs so SOURCEDIR/usr/bin needs to be stored as /usr/bin
		// in the cpio image
		rec.Name = strings.TrimPrefix(rec.Name, sourceDir)

		if err := rw.WriteRecord(cpio.MakeReproducible(rec)); err != nil {
			return fmt.Errorf("writing record %q failed: %w", filePath, err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("error walking the source dir: %w", err)
	}

	if err := cpio.WriteTrailer(rw); err != nil {
		return fmt.Errorf("error writing trailer record: %w", err)
	}

	b.logger.Info("Compressing initramfs")

	if err := ZstdFile(cpioFileName, filepath.Join(artifactsTempDir, "initrd")); err != nil {
		return err
	}

	if err := os.RemoveAll(cpioFileName); err != nil {
		return fmt.Errorf("error deleting cpio file: %w", err)
	}

	return nil
}

func (b *BuildUKIAction) copyKernel(sourceDir, targetDir string) error {
	linkTarget, err := os.Readlink(filepath.Join(sourceDir, "boot", "vmlinuz"))
	if err != nil {
		return err
	}

	kernelFile := filepath.Base(linkTarget)
	sourceFile, err := os.Open(filepath.Join(sourceDir, "boot", kernelFile))
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(filepath.Join(targetDir, "vmlinuz"))
	if err != nil {
		return err
	}
	defer destinationFile.Close()
	b.logger.Infof("Copying kernel from: %s to: %s", sourceFile.Name(), destinationFile.Name())
	_, err = io.Copy(destinationFile, sourceFile)

	return err
}

func (b *BuildUKIAction) createConfFiles(sourceDir, cmdline, title, finalEfiName string) error {
	// This is stored in the config
	var extraCmdline string
	// For the config title we get only the extra cmdline we added, no replacement of spaces with underscores needed
	extraCmdline = strings.TrimSpace(strings.TrimPrefix(cmdline, constants.UkiCmdline))
	// For the default install entry, do not add anything on the config
	if extraCmdline == constants.UkiCmdlineInstall {
		extraCmdline = ""
	}
	b.logger.Infof("Creating the %s.conf file", finalEfiName)

	// You can add entries into the config files, they will be ignored by systemd-boot
	// So we store the cmdline in a key cmdline for easy tracking of what was added to the uki cmdline

	configData := fmt.Sprintf("title %s\nefi /EFI/kairos/%s.efi\n", title, finalEfiName)

	if viper.GetBool("include-version-in-config") {
		configData = fmt.Sprintf("%sversion %s\n", configData, b.version)
	}

	if viper.GetBool("include-cmdline-in-config") {
		configData = fmt.Sprintf("%scmdline %s\n", configData, strings.Trim(extraCmdline, " "))
	}

	err := os.WriteFile(filepath.Join(sourceDir, finalEfiName+".conf"), []byte(configData), os.ModePerm)
	if err != nil {
		return fmt.Errorf("creating the %s.conf file", finalEfiName)
	}

	return nil
}

func (b *BuildUKIAction) createISO(sourceDir string) error {
	// isoDir is where we generate the img file. We pass this dir to xorriso.
	isoDir, err := os.MkdirTemp("", "enki-iso-dir-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(isoDir)

	filesMap, err := b.imageFiles(sourceDir)
	if err != nil {
		return err
	}

	b.logger.Info("Calculating the size of the img file")
	artifactSize, err := sumFileSizes(filesMap)
	if err != nil {
		return err
	}

	// Create just the size we need + 50MB just in case
	imgSize := artifactSize + 50
	imgFile := filepath.Join(isoDir, "efiboot.img")
	b.logger.Info(fmt.Sprintf("Creating the img file with size: %dMb", imgSize))
	if err = createImgWithSize(imgFile, imgSize); err != nil {
		return err
	}
	defer os.Remove(imgFile)

	b.logger.Info(fmt.Sprintf("Created image: %s", imgFile))

	b.logger.Info("Creating directories in the img file")
	if err := createImgDirs(imgFile, filesMap); err != nil {
		return err
	}

	b.logger.Info("Copying files in the img file")
	if err := copyFilesToImg(imgFile, filesMap); err != nil {
		return err
	}

	if viper.GetString("overlay-iso") != "" {
		b.logger.Infof("Adding files from %s to iso", viper.GetString("overlay-iso"))
		overlay, err := v1.NewSrcFromURI(fmt.Sprintf("dir:%s", viper.GetString("overlay-iso")))
		if err != nil {
			b.logger.Errorf("error creating overlay image: %s", err)
			return err
		}
		_, err = b.e.DumpSource(isoDir, overlay)

		if err != nil {
			b.logger.Errorf("error copying overlay image: %s", err)
			return err
		}

	}

	isoName := fmt.Sprintf("kairos_%s.iso", b.version)
	if b.name != "" {
		isoName = fmt.Sprintf("%s.iso", b.name)
	}

	b.logger.Info("Creating the iso files with xorriso")
	cmd := exec.Command("xorriso", "-as", "mkisofs", "-V", "UKI_ISO_INSTALL", "-isohybrid-gpt-basdat",
		"-e", filepath.Base(imgFile), "-no-emul-boot", "-o", filepath.Join(b.outputDir, isoName), isoDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error creating iso file: %w\n%s", err, string(out))
	}

	return nil
}

func (b *BuildUKIAction) createContainer(sourceDir, version string) error {
	temp, err := os.CreateTemp("", "image.tar")
	if err != nil {
		return err
	}
	// Create tarball from sourceDir
	err = utils.Tar(sourceDir, temp)
	if err != nil {
		return err
	}
	_ = temp.Close()
	defer os.RemoveAll(temp.Name())
	finalImage := filepath.Join(b.outputDir, fmt.Sprintf("kairos_uki_%s.tar", version))
	// TODO: get the arch from the running system or by flag? Config.Arch has this value on it
	arch := "amd64"
	os := "linux"
	// Build imageTar from normal tar
	tarName := fmt.Sprintf("kairos_uki_%s.tar", version)
	if b.name != "" {
		tarName = fmt.Sprintf("%s.tar", b.name)
	}
	err = utils.CreateTar(b.logger, temp.Name(), finalImage, tarName, arch, os)
	if err != nil {
		return err
	}
	b.logger.Infof("Done building %s at: %s", b.outputType, finalImage)

	return err
}

// Create artifact just outputs the files from the sourceDir to the outputDir
// Maintains the same structure as the sourceDir which is the final structure we want
func (b *BuildUKIAction) createArtifact(sourceDir string) error {
	filesMap, err := b.imageFiles(sourceDir)
	if err != nil {
		return err
	}
	for dir, files := range filesMap {
		b.logger.Debugf(fmt.Sprintf("creating dir %s", filepath.Join(b.outputDir, dir)))
		err = os.MkdirAll(filepath.Join(b.outputDir, dir), os.ModeDir|os.ModePerm)
		if err != nil {
			b.logger.Errorf("creating dir %s: %s", dir, err)
			return err
		}
		for _, f := range files {
			b.logger.Debugf(fmt.Sprintf("copying %s to %s", f, filepath.Join(b.outputDir, dir, filepath.Base(f))))
			source, err := os.Open(f)
			if err != nil {
				b.logger.Errorf("opening file %s: %s", f, err)
				return err
			}
			defer func(source *os.File) {
				err := source.Close()
				if err != nil {
					b.logger.Errorf("closing file %s: %s", f, err)
				}
			}(source)

			destination, err := os.Create(filepath.Join(b.outputDir, dir, filepath.Base(f)))
			if err != nil {
				b.logger.Errorf("creating file %s: %s", filepath.Join(b.outputDir, dir, filepath.Base(f)), err)
				return err
			}
			defer func(destination *os.File) {
				err := destination.Close()
				if err != nil {
					b.logger.Errorf("closing file %s: %s", filepath.Join(b.outputDir, dir, filepath.Base(f)), err)
				}
			}(destination)
			_, err = io.Copy(destination, source)
			if err != nil {
				b.logger.Errorf("copying file %s: %s", f, err)
				return err
			}
		}
	}
	return nil
}

func (b *BuildUKIAction) imageFiles(sourceDir string) (map[string][]string, error) {
	// the keys are the target dirs
	// the values are the source files that should be copied into the target dir
	data := map[string][]string{
		"EFI":            {},
		"EFI/BOOT":       {filepath.Join(sourceDir, "BOOTX64.EFI")},
		"EFI/kairos":     {},
		"EFI/tools":      {},
		"loader":         {filepath.Join(sourceDir, "loader.conf")},
		"loader/entries": {},
		"loader/keys":    {},
		"loader/keys/auto": {
			filepath.Join(b.keysDirectory, "PK.der"),
			filepath.Join(b.keysDirectory, "KEK.der"),
			filepath.Join(b.keysDirectory, "db.der"),
			filepath.Join(b.keysDirectory, "PK.auth"),
			filepath.Join(b.keysDirectory, "KEK.auth"),
			filepath.Join(b.keysDirectory, "db.auth")},
	}
	// Add the kairos efi files and the loader conf files for each cmdline
	entries := append(utils.GetUkiCmdline(), utils.GetUkiSingleCmdlines(b.logger)...)
	for _, entry := range entries {
		data["EFI/kairos"] = append(data["EFI/kairos"], filepath.Join(sourceDir, entry.FileName+".efi"))
		data["loader/entries"] = append(data["loader/entries"], filepath.Join(sourceDir, entry.FileName+".conf"))
	}
	return data, nil
}

// removeUkiFiles removes all the files and directories inside the output directory that match our filesMap
// so this should only remove the generated intermediate artifacts that we use to build the container
func (b *BuildUKIAction) removeUkiFiles() error {
	filesMap, _ := b.imageFiles(b.outputDir)
	for dir, files := range filesMap {
		for _, f := range files {
			err := os.Remove(filepath.Join(b.outputDir, dir, filepath.Base(f)))
			if err != nil {
				return err
			}
		}
	}
	for dir := range filesMap {
		err := os.RemoveAll(filepath.Join(b.outputDir, dir))
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *BuildUKIAction) getEfiStub() (string, error) {
	if utils.IsAmd64(b.arch) {
		return constants.UkiSystemdBootStubx86, nil
	} else if utils.IsArm64(b.arch) {
		return constants.UkiSystemdBootStubArm, nil
	} else {
		return "", nil
	}
}

func (b *BuildUKIAction) getEfiNeededFiles() ([]string, error) {
	if utils.IsAmd64(b.arch) {
		return []string{
			constants.UkiSystemdBootStubx86,
			constants.UkiSystemdBootx86,
		}, nil
	} else if utils.IsArm64(b.arch) {
		return []string{
			constants.UkiSystemdBootStubArm,
			constants.UkiSystemdBootArm,
		}, nil
	} else {
		return nil, fmt.Errorf("unsupported arch: %s", b.arch)
	}
}

func (b *BuildUKIAction) cleanSource(dir string) {
	// Remove the boot directory as we already copied the kernel and we dont need the initrd files
	err := os.RemoveAll(filepath.Join(dir, "boot"))
	if err != nil {
		b.logger.Errorf("removing boot dir: %s", err)
		return
	}
	// TODO: there should be a copy of the kernel at /usrt/lib/modules/VERSION/kernel/vmlinuz that we may also want to remove
}

func copyFilesToImg(imgFile string, filesMap map[string][]string) error {
	for dir, files := range filesMap {
		for _, f := range files {
			cmd := exec.Command("mcopy", "-i", imgFile, f, filepath.Join(fmt.Sprintf("::%s", dir), filepath.Base(f)))
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("copying %s in img file: %w\n%s", f, err, string(out))
			}
		}
	}

	return nil
}

func ZstdFile(sourcePath, targetPath string) error {
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("error opening initramfs file: %w", err)
	}
	defer inputFile.Close()

	outputFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("error creating compressed initramfs file: %w", err)
	}
	defer outputFile.Close()

	// SpeedBetterCompression is heavier, takes 36 seconds in my 24core cpu but generates a 919MB file
	// SpeedBestCompression is really fast, takes 6 seconds but generates a 950Mb file
	// If we need we can use the heavier one if we need to gain those 30 extra Mb
	zstdWriter, _ := zstd.NewWriter(outputFile, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	defer zstdWriter.Close()

	if _, err = io.Copy(zstdWriter, inputFile); err != nil {
		return fmt.Errorf("error writing data to the compress initramfs file: %w", err)
	}

	return nil
}

func findKairosVersion(sourceDir string) (string, error) {
	osReleaseBytes, err := os.ReadFile(filepath.Join(sourceDir, "etc", "os-release"))
	if err != nil {
		return "", fmt.Errorf("reading os-release file: %w", err)
	}

	re := regexp.MustCompile("(?m)^KAIROS_RELEASE=\"(.*)\"")
	match := re.FindStringSubmatch(string(osReleaseBytes))

	if len(match) != 2 {
		return "", fmt.Errorf("unexpected number of matches for KAIROS_RELEASE in os-release: %d", len(match))
	}

	return match[1], nil
}

func createImgWithSize(imgFile string, size int64) error {
	cmd := exec.Command("dd",
		"if=/dev/zero", fmt.Sprintf("of=%s", imgFile),
		"bs=1M", fmt.Sprintf("count=%d", size),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("creating the img file: %w\n%s", err, out)
	}

	return nil
}

func sumFileSizes(filesMap map[string][]string) (int64, error) {
	total := int64(0)
	for _, files := range maps.Values(filesMap) {
		for _, f := range files {
			fileInfo, err := os.Stat(f)
			if err != nil {
				return total, fmt.Errorf("finding file info for file %s: %w", f, err)
			}
			total += fileInfo.Size()
		}
	}

	totalInMB := int64(math.Round(float64(total) / (1024 * 1024)))

	return totalInMB, nil
}

func createImgDirs(imgFile string, filesMap map[string][]string) error {
	cmd := exec.Command("mkfs.msdos", "-F", "32", imgFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("formating the img file to fat: %w\n%s", err, string(out))
	}

	dirs := maps.Keys(filesMap)
	sort.Strings(dirs) // Make sure we create outer dirs first
	for _, dir := range dirs {
		// Dirs in MSDOS are marked with ::DIR
		cmd := exec.Command("mmd", "-i", imgFile, fmt.Sprintf("::%s", dir))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("creating directory %s on the img file: %w\n%s\nThe failed command was: %s", dir, err, string(out), cmd.String())
		}
	}

	return nil
}
