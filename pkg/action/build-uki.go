package action

import (
	"compress/gzip"
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
	"github.com/spf13/viper"

	"github.com/sanity-io/litter"
	"github.com/u-root/u-root/pkg/cpio"
	"golang.org/x/exp/maps"

	"github.com/kairos-io/enki/pkg/types"
	"github.com/kairos-io/enki/pkg/utils"
	"github.com/kairos-io/kairos-agent/v2/pkg/elemental"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
)

type BuildUKIAction struct {
	img           *v1.ImageSource
	e             *elemental.Elemental
	outputDir     string
	keysDirectory string
	logger        v1.Logger
	outputType    string
	version       string
}

func NewBuildUKIAction(cfg *types.BuildConfig, img *v1.ImageSource, outputDir, keysDirectory, outputType string) *BuildUKIAction {
	b := &BuildUKIAction{
		logger:        cfg.Logger,
		img:           img,
		e:             elemental.NewElemental(&cfg.Config),
		outputDir:     outputDir,
		keysDirectory: keysDirectory,
		outputType:    outputType,
	}
	b.logger.Debugf("BuildUKIAction: %+v", litter.Sdump(b))
	return b
}

func (b *BuildUKIAction) Run() error {
	err := b.checkDeps()
	if err != nil {
		return err
	}

	b.logger.Info("Extracting image to a temporary directory")
	sourceDir, err := b.extractImage()
	if err != nil {
		return err
	}
	defer os.RemoveAll(sourceDir)

	if viper.GetString("overlay") != "" {
		b.logger.Infof("Adding files from %s to rootfs", viper.GetString("overlay"))
		overlay, err := v1.NewSrcFromURI(fmt.Sprintf("dir:%s", viper.GetString("overlay")))
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

	b.logger.Info("Creating an initramfs file")
	if err := b.createInitramfs(sourceDir); err != nil {
		return err
	}

	cmdlines := utils.GetUkiCmdline()
	for _, cmdline := range cmdlines {
		b.logger.Info("Running ukify for cmdline: " + cmdline)
		if err := b.ukify(sourceDir, cmdline); err != nil {
			return err
		}
		b.logger.Info("Creating kairos and loader conf files")
		if err := b.createConfFiles(sourceDir, cmdline); err != nil {
			return err
		}
	}

	err = b.createSystemdConf(sourceDir)
	if err != nil {
		return err
	}

	b.logger.Info("Signing artifacts")
	if err := b.sbSign(sourceDir); err != nil {
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
		finalEfiConf = nameFromCmdline(b.version, constants.UkiCmdline+" "+constants.UkiCmdlineInstall) + ".conf"
	}
	// Set that as default selection for booting
	data := fmt.Sprintf("default %s\ntimeout 5\nconsole-mode max\neditor no\n", finalEfiConf)
	err := os.WriteFile(filepath.Join(sourceDir, "loader.conf"), []byte(data), os.ModePerm)
	if err != nil {
		return fmt.Errorf("creating the loader.conf file: %s", err)
	}
	return nil
}

func (b *BuildUKIAction) extractImage() (string, error) {
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
		"/usr/lib/systemd/ukify",
		"sbsign",
		"dd",
		"mkfs.msdos",
		"mmd",
		"mcopy",
		"xorriso",
	}

	neededFiles := []string{
		"/usr/lib/systemd/boot/efi/linuxx64.efi.stub",
		"/usr/lib/systemd/boot/efi/systemd-bootx64.efi",
	}

	for _, b := range neededBinaries {
		_, err := exec.LookPath(b)
		if err != nil {
			return err
		}
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
func (b *BuildUKIAction) createInitramfs(sourceDir string) error {
	format := "newc"
	archiver, err := cpio.Format(format)
	if err != nil {
		return fmt.Errorf("format %q not supported: %w", format, err)
	}

	cpioFileName := filepath.Join(sourceDir, "initramfs.cpio")
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

		rec.Name = strings.TrimPrefix(rec.Name, sourceDir)
		if err := rw.WriteRecord(rec); err != nil {
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

	if err := GzipFile(cpioFileName, "initrd"); err != nil {
		return err
	}

	if err := os.RemoveAll(cpioFileName); err != nil {
		return fmt.Errorf("error deleting cpio file: %w", err)
	}

	return nil
}

func (b *BuildUKIAction) uname(sourceDir string) (string, error) {
	files, err := filepath.Glob(filepath.Join(sourceDir, "boot", "vmlinuz-*"))
	if err != nil {
		return "", fmt.Errorf("getting file list: %w", err)
	}

	matchingFile := ""
	for _, file := range files {
		if !strings.Contains(file, "rescue") {
			matchingFile = file
			break
		}
	}
	if matchingFile == "" {
		return "", fmt.Errorf("no matching vmlinuz file found")
	}

	// Extract the basename and remove "vmlinuz-" using a regular expression
	re := regexp.MustCompile(`vmlinuz-(.+)`)
	match := re.FindStringSubmatch(filepath.Base(matchingFile))
	if len(match) <= 1 {
		return "", fmt.Errorf("error extracting uname")
	}

	return match[1], nil
}

func (b *BuildUKIAction) copyKernel(sourceDir string) error {
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

	destinationFile, err := os.Create(filepath.Join(sourceDir, "vmlinuz"))
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)

	return err
}

func (b *BuildUKIAction) ukify(sourceDir, cmdline string) error {
	// Normally that's still the current dir but just making sure.
	if err := os.Chdir(sourceDir); err != nil {
		return fmt.Errorf("changing to %s directory: %w", sourceDir, err)
	}

	uname, err := b.uname(sourceDir)
	if err != nil {
		return err
	}

	if err := b.copyKernel(sourceDir); err != nil {
		return err
	}

	// name: kairosVersion + cmdline + .efi
	// for the cmdline, we remove the shared cmdline values and only keep the extra values added by the user
	// we also replace spaces with underscores
	cmdlineName := strings.TrimSpace(strings.TrimPrefix(cmdline, constants.UkiCmdline))
	// For the default install entry, do not add anything on the efi name
	if cmdlineName == constants.UkiCmdlineInstall {
		cmdlineName = ""
	}
	finalEfiName := strings.TrimSuffix(b.version+"_"+strings.Replace(cmdlineName, " ", "_", -1), "_") + ".efi"
	b.logger.Infof("Generating: " + finalEfiName)
	cmd := exec.Command("/usr/lib/systemd/ukify",
		"--linux", "vmlinuz",
		"--initrd", "initrd",
		"--cmdline", cmdline,
		"--os-release", fmt.Sprintf("@%s", "etc/os-release"),
		"--uname", uname,
		"--stub", "/usr/lib/systemd/boot/efi/linuxx64.efi.stub",
		"--secureboot-private-key", filepath.Join(b.keysDirectory, "DB.key"),
		"--secureboot-certificate", filepath.Join(b.keysDirectory, "DB.pem"),
		"--pcr-private-key", filepath.Join(b.keysDirectory, "tpm2-pcr-private.pem"),
		"--measure",
		"--output", finalEfiName,
		"build",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("running ukify: %w\n%s", err, string(out))
	}
	return nil
}

// TODO: the efi file should come from the downloaded image, not from the
// enki running OS.
func (b *BuildUKIAction) sbSign(sourceDir string) error {
	cmd := exec.Command("sbsign",
		"--key", filepath.Join(b.keysDirectory, "DB.key"),
		"--cert", filepath.Join(b.keysDirectory, "DB.pem"),
		"--output", filepath.Join(sourceDir, "BOOTX64.EFI"),
		"/usr/lib/systemd/boot/efi/systemd-bootx64.efi",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("running sbsign: %w\n%s", err, string(out))
	}
	return nil
}

func (b *BuildUKIAction) createConfFiles(sourceDir, cmdline string) error {
	// This is for the UI only, what it shows on the menu to select
	var cmdlineForConf string
	// This is stored in the config
	var extraCmdline string
	finalEfiName := nameFromCmdline(b.version, cmdline)
	// For the config title we get only the extra cmdline we added, no replacement of spaces with underscores needed
	extraCmdline = strings.TrimSpace(strings.TrimPrefix(cmdline, constants.UkiCmdline))
	// For the default install entry, do not add anything on the config
	if extraCmdline == constants.UkiCmdlineInstall {
		extraCmdline = ""
	}
	// Add some  ( ) around the extra cmdline if it's not empty for a nicer display
	if extraCmdline != "" {
		cmdlineForConf = fmt.Sprintf("(%s)", strings.Trim(extraCmdline, " "))
	} else {
		// Empty extra cmdline, we don't want to display anything
		cmdlineForConf = extraCmdline
	}
	b.logger.Infof("Creating the %s.conf file", finalEfiName)

	title := viper.GetString("boot-branding")
	// You can add entries into the config files, they will be ignored by systemd-boot
	// So we store the cmdline in a key cmdline for easy tracking of what was added to the uki cmdline
	data := fmt.Sprintf("title %s %s %s\nefi /EFI/kairos/%s.efi\nversion %s\ncmdline %s\n", title, b.version, cmdlineForConf, finalEfiName, b.version, strings.Trim(cmdline, " "))
	err := os.WriteFile(filepath.Join(sourceDir, finalEfiName+".conf"), []byte(data), os.ModePerm)
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

	isoName := fmt.Sprintf("kairos_%s.iso", b.version)

	b.logger.Info("Creating the iso files with xorriso")
	cmd := exec.Command("xorriso", "-as", "mkisofs", "-V", "UKI_ISO_INSTALL",
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
	err = utils.CreateTar(b.logger, temp.Name(), finalImage, fmt.Sprintf("kairos_uki:%s", version), arch, os)
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
			filepath.Join(b.keysDirectory, "DB.der"),
			filepath.Join(b.keysDirectory, "PK.auth"),
			filepath.Join(b.keysDirectory, "KEK.auth"),
			filepath.Join(b.keysDirectory, "DB.auth")},
	}
	// Add the kairos efi files and the loader conf files for each cmdline
	cmdlines := utils.GetUkiCmdline()
	for _, cmdline := range cmdlines {
		finalEfiName := nameFromCmdline(b.version, cmdline)
		data["EFI/kairos"] = append(data["EFI/kairos"], filepath.Join(sourceDir, finalEfiName+".efi"))
		data["loader/entries"] = append(data["loader/entries"], filepath.Join(sourceDir, finalEfiName+".conf"))
	}
	b.logger.Debug(fmt.Sprintf("data: %s", litter.Sdump(data)))
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
	for dir, _ := range filesMap {
		err := os.RemoveAll(filepath.Join(b.outputDir, dir))
		if err != nil {
			return err
		}
	}
	return nil
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

func GzipFile(sourcePath, targetPath string) error {
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

	gzipWriter := gzip.NewWriter(outputFile)
	defer gzipWriter.Close()

	if _, err = io.Copy(gzipWriter, inputFile); err != nil {
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

// nameFromCmdline returns the name of the efi/conf file based on the cmdline
// we want to have at least 1 efi file that its the default, that is the one we ship with the iso/media/whatever install medium
// that one has the default cmdline + the install cmdline
// For that one, we use it as the BASE one, configs will only trigger for that install stanza if we are on install media
// so we dont have to worry about it, but we want to provide a clean name for it
// so in that case we dont add anything to the efi name/conf name/cmdline inside the config
// For the other ones, we add the cmdline to the efi name and the cmdline to the conf file
// so you get
// - v3.0.0-alpha1.efi
// - v3.0.0-alpha1.conf
// - v3.0.0-alpha1_interactive-install.efi
// - v3.0.0-alpha1_interactive-install.conf
// This is mostly for convenience in generating the names as the real data is stored in the config file
// but it can easily be used to identify the efi file and the conf file
// FULLVERSION_CMDLINE is the format so we can split on _ and get the version and the cmdline separated.
// Spaces in the cmdline are replaced with underscores, so we cant get the cmdline from the name easily
// that why we store it on the config file, along with the version
func nameFromCmdline(version, cmdline string) string {
	// Remove the default cmdline from the current cmdline
	cmdlineForEfi := strings.TrimSpace(strings.TrimPrefix(cmdline, constants.UkiCmdline))
	// For the default install entry, do not add anything on the efi name
	if cmdlineForEfi == constants.UkiCmdlineInstall {
		cmdlineForEfi = ""
	}
	// Change spaces to underscores
	cleanCmdline := strings.Replace(cmdlineForEfi, " ", "_", -1)
	// Firs step we get a version + underscore to separete the version from the cmdline and cmdline
	name := version + "_" + cleanCmdline
	// If the cmdline is empty, we remove the underscore as to not get a dangling one
	finalName := strings.TrimSuffix(name, "_")
	return finalName
}
