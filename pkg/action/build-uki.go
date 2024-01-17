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
	"strings"

	"github.com/u-root/u-root/pkg/cpio"
	"golang.org/x/exp/maps"

	"github.com/kairos-io/enki/pkg/types"
	"github.com/kairos-io/kairos-agent/v2/pkg/elemental"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
)

const Cmdline = "console=ttyS0 console=tty1 net.ifnames=1 rd.immucore.oemlabel=COS_OEM rd.immucore.debug rd.immucore.oemtimeout=2 rd.immucore.uki selinux=0"

type BuildUKIAction struct {
	img           *v1.ImageSource
	e             *elemental.Elemental
	isoFile       string
	keysDirectory string
	logger        v1.Logger
}

func NewBuildUKIAction(cfg *types.BuildConfig, img *v1.ImageSource, result, keysDirectory string) *BuildUKIAction {
	b := &BuildUKIAction{
		logger:        cfg.Logger,
		img:           img,
		e:             elemental.NewElemental(&cfg.Config),
		isoFile:       result,
		keysDirectory: keysDirectory,
	}
	return b
}

func (b *BuildUKIAction) Run() error {
	err := b.checkDeps()
	if err != nil {
		return err
	}

	b.logger.Info("extracting image to a temporary directory")
	sourceDir, err := b.extractImage()
	if err != nil {
		return err
	}
	defer os.RemoveAll(sourceDir)

	b.logger.Info("creating additional directories")
	if err := b.setupDirectoriesAndFiles(sourceDir); err != nil {
		return err
	}

	b.logger.Info("creating an initramfs file")
	if err := b.createInitramfs(sourceDir); err != nil {
		return err
	}

	b.logger.Info("running ukify")
	if err := b.ukify(sourceDir); err != nil {
		return err
	}

	b.logger.Info("running sgsign")
	if err := b.sbSign(sourceDir); err != nil {
		return err
	}

	b.logger.Info("creating kairos and loader conf files")
	if err := b.createConfFiles(sourceDir); err != nil {
		return err
	}

	if err := b.createISO(sourceDir); err != nil {
		return err
	}

	return nil
}

func (b *BuildUKIAction) extractImage() (string, error) {
	tmpDir, err := os.MkdirTemp("", "enki-build-uki-")
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
		// TODO: this should come from the given image, not the OS where enki runs
		"/usr/lib/systemd/boot/efi/linuxx64.efi.stub",
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
		"./sys":  true,
		"./run":  true,
		"./dev":  true,
		"./tmp":  true,
		"./proc": true,
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
		if excludeDirs[filePath] {
			return filepath.SkipDir
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

func (b *BuildUKIAction) ukify(sourceDir string) error {
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

	kairosVersion, err := findKairosVersion(sourceDir)
	if err != nil {
		return err
	}

	cmd := exec.Command("/usr/lib/systemd/ukify",
		"--linux", "vmlinuz",
		"--initrd", "initrd",
		"--cmdline", Cmdline,
		"--os-release", fmt.Sprintf("@%s", "etc/os-release"),
		"--uname", uname,
		"--stub", "/usr/lib/systemd/boot/efi/linuxx64.efi.stub",
		"--secureboot-private-key", filepath.Join(b.keysDirectory, "DB.key"),
		"--secureboot-certificate", filepath.Join(b.keysDirectory, "DB.crt"),
		"--pcr-private-key", filepath.Join(b.keysDirectory, "tpm2-pcr-private.pem"),
		"--measure",
		"--output", filepath.Join(sourceDir, kairosVersion+".efi"),
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
		"--cert", filepath.Join(b.keysDirectory, "DB.crt"),
		"--output", filepath.Join(sourceDir, "BOOTX64.EFI"),
		"/usr/lib/systemd/boot/efi/systemd-bootx64.efi",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("running sbsign: %w\n%s", err, string(out))
	}
	return nil
}

func (b *BuildUKIAction) createConfFiles(sourceDir string) error {
	kairosVersion, err := findKairosVersion(sourceDir)
	if err != nil {
		return err
	}
	data := fmt.Sprintf("title Kairos %[1]s\nefi /EFI/kairos/%[1]s.efi\nversion %[1]s", kairosVersion)
	err = os.WriteFile(filepath.Join(sourceDir, kairosVersion+".conf"), []byte(data), os.ModePerm)
	if err != nil {
		return fmt.Errorf("creating the %s.conf file", kairosVersion)
	}

	data = "default @saved\ntimeout 5\nconsole-mode max\neditor no\n"
	err = os.WriteFile(filepath.Join(sourceDir, "loader.conf"), []byte(data), os.ModePerm)
	if err != nil {
		return fmt.Errorf("creating the loader.conf file")
	}

	return nil
}

func (b *BuildUKIAction) createISO(sourceDir string) error {
	tmpDir, err := os.MkdirTemp("", "enki-iso-dir-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	filesMap, err := b.imageFiles(sourceDir)
	if err != nil {
		return err
	}

	b.logger.Info("preparing a directory with the contents of the ISO")
	if err := prepareISODir(filesMap, tmpDir); err != nil {
		return err
	}

	b.logger.Info("calculating the size of the img file")
	artifactSize, err := sumFileSizes(tmpDir)
	if err != nil {
		return err
	}

	// Create just the size we need + 50MB just in case
	imgSize := artifactSize + 50
	b.logger.Info(fmt.Sprintf("creating the img file with size: %dMb", imgSize))
	imgFile, err := createImgWithSize(imgSize)
	if err != nil {
		return err
	}
	defer os.Remove(imgFile)

	b.logger.Info("creating directories in the img file")
	if err := createImgDirs(imgFile, filesMap); err != nil {
		return err
	}

	b.logger.Info("copying files in the img file")
	if err := copyFilesToImg(imgFile, filesMap); err != nil {
		return err
	}

	b.logger.Info("creating the iso files with xorriso")
	cmd := exec.Command("xorriso", "-as", "mkisofs", "-V", "UKI_ISO_INSTALL",
		"-e", imgFile, "-no-emul-boot", "-o", b.isoFile, tmpDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error creating iso file: %w\n%s", err, string(out))
	}

	return nil
}

func (b *BuildUKIAction) imageFiles(sourceDir string) (map[string][]string, error) {
	kairosVersion, err := findKairosVersion(sourceDir)
	if err != nil {
		return map[string][]string{}, err
	}

	// the keys are the target dirs
	// the values are the source files that should be copied into the target dir
	return map[string][]string{
		"::EFI":            {},
		"::EFI/BOOT":       {filepath.Join(sourceDir, "BOOTX64.EFI")},
		"::EFI/kairos":     {filepath.Join(sourceDir, kairosVersion+".efi")},
		"::EFI/tools":      {},
		"::loader":         {filepath.Join(sourceDir, "loader.conf")},
		"::loader/entries": {filepath.Join(sourceDir, kairosVersion+".conf")},
		"::loader/keys":    {},
		"::loader/keys/kairos": {
			filepath.Join(b.keysDirectory, "PK.der"),
			filepath.Join(b.keysDirectory, "KEK.der"),
			filepath.Join(b.keysDirectory, "DB.der")},
	}, nil
}

func copyFilesToImg(imgFile string, filesMap map[string][]string) error {
	for dir, files := range filesMap {
		for _, f := range files {
			cmd := exec.Command("mcopy", "-i", imgFile, f, filepath.Join(dir, filepath.Base(f)))
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

func createImgWithSize(size int64) (string, error) {
	result, err := os.CreateTemp("", "enki-uki-*.img")
	if err != nil {
		return "", fmt.Errorf("creating a temporary img file: %w", err)
	}

	cmd := exec.Command("dd",
		"if=/dev/zero",
		fmt.Sprintf("of=%s", result.Name()),
		"bs=1M",
		fmt.Sprintf("count=%d", size),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("creating the img file: %w\n%s", err, out)
	}

	return result.Name(), nil
}

func sumFileSizes(dir string) (int64, error) {
	total := int64(0)
	err := filepath.Walk(dir, func(filePath string, fileInfo os.FileInfo, err error) error {
		total += fileInfo.Size()
		return nil
	})

	totalInMB := int64(math.Round(float64(total) / (1024 * 1024)))

	return totalInMB, err
}

// prepareISODir copies all the needed files for the iso from the sourceDir
// to the tmpDir
func prepareISODir(filesMap map[string][]string, targetDir string) error {
	for _, files := range maps.Values(filesMap) {
		for _, f := range files {
			sourceFile, err := os.Open(f)
			if err != nil {
				return err
			}
			defer sourceFile.Close()

			destinationFile, err := os.Create(filepath.Join(targetDir, filepath.Base(f)))
			if err != nil {
				return err
			}
			defer destinationFile.Close()

			_, err = io.Copy(destinationFile, sourceFile)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func createImgDirs(imgFile string, filesMap map[string][]string) error {
	cmd := exec.Command("mkfs.msdos", "-F", "32", imgFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("formating the img file to fat: %w\n%s", err, string(out))
	}

	dirs := maps.Keys(filesMap)
	for _, dir := range dirs {
		cmd := exec.Command("mmd", "-i", imgFile, dir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("creating directory %s on the img file: %w\n%s\nThe failed command was: %s", dir, err, string(out), cmd.String())
		}
	}

	return nil
}
