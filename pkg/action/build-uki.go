package action

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/u-root/u-root/pkg/cpio"

	"github.com/kairos-io/enki/pkg/types"
	"github.com/kairos-io/kairos-agent/v2/pkg/elemental"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
)

const Cmdline = "console=ttyS0 console=tty1 net.ifnames=1 rd.immucore.oemlabel=COS_OEM rd.immucore.debug rd.immucore.oemtimeout=2 rd.immucore.uki selinux=0"

type BuildUKIAction struct {
	img           *v1.ImageSource
	e             *elemental.Elemental
	ukiFile       string
	keysDirectory string
}

func NewBuildUKIAction(cfg *types.BuildConfig, img *v1.ImageSource, result, keysDirectory string) *BuildUKIAction {
	b := &BuildUKIAction{
		img:           img,
		e:             elemental.NewElemental(&cfg.Config),
		ukiFile:       result,
		keysDirectory: keysDirectory,
	}
	return b
}

func (b *BuildUKIAction) Run() error {
	err := b.checkDeps()
	if err != nil {
		return err
	}

	tmpDir, err := b.extractImage()
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := b.setupDirectoriesAndFiles(tmpDir); err != nil {
		return err
	}

	if err := b.createInitramfs(tmpDir); err != nil {
		return err
	}

	if err := b.ukify(tmpDir); err != nil {
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
		"--output", filepath.Join(sourceDir, "uki.signed.efi"),
		"build",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("running ukify: %w\n%s", err, string(out))
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
