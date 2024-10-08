package action

import (
	"fmt"
	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/filesystem/squashfs"
	"github.com/kairos-io/enki/pkg/constants"
	"github.com/kairos-io/enki/pkg/types"
	"github.com/kairos-io/enki/pkg/utils"
	"github.com/kairos-io/kairos-agent/v2/pkg/elemental"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
	sdk "github.com/kairos-io/kairos-sdk/utils"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BuildISOAction struct {
	cfg  *types.BuildConfig
	spec *types.LiveISO
	e    *elemental.Elemental
}

type BuildISOActionOption func(a *BuildISOAction)

func NewBuildISOAction(cfg *types.BuildConfig, spec *types.LiveISO, opts ...BuildISOActionOption) *BuildISOAction {
	b := &BuildISOAction{
		cfg:  cfg,
		e:    elemental.NewElemental(&cfg.Config),
		spec: spec,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// ISORun will install the system from a given configuration
func (b *BuildISOAction) ISORun() (err error) {
	cleanup := sdk.NewCleanStack()
	defer func() { err = cleanup.Cleanup(err) }()

	isoTmpDir, err := utils.TempDir(b.cfg.Fs, "", "enki-iso")
	if err != nil {
		return err
	}
	cleanup.Push(func() error { return b.cfg.Fs.RemoveAll(isoTmpDir) })

	rootDir := filepath.Join(isoTmpDir, "rootfs")
	err = utils.MkdirAll(b.cfg.Fs, rootDir, constants.DirPerm)
	if err != nil {
		return err
	}

	uefiDir := filepath.Join(isoTmpDir, "uefi")
	err = utils.MkdirAll(b.cfg.Fs, uefiDir, constants.DirPerm)
	if err != nil {
		return err
	}

	isoDir := filepath.Join(isoTmpDir, "iso")
	err = utils.MkdirAll(b.cfg.Fs, isoDir, constants.DirPerm)
	if err != nil {
		return err
	}

	if b.cfg.OutDir != "" {
		err = utils.MkdirAll(b.cfg.Fs, b.cfg.OutDir, constants.DirPerm)
		if err != nil {
			b.cfg.Logger.Errorf("Failed creating output folder: %s", b.cfg.OutDir)
			return err
		}
	}

	b.cfg.Logger.Infof("Preparing squashfs root...")
	err = b.applySources(rootDir, b.spec.RootFS...)
	if err != nil {
		b.cfg.Logger.Errorf("Failed installing OS packages: %v", err)
		return err
	}
	err = utils.CreateDirStructure(b.cfg.Fs, rootDir)
	if err != nil {
		b.cfg.Logger.Errorf("Failed creating root directory structure: %v", err)
		return err
	}

	b.cfg.Logger.Infof("Preparing ISO image root tree...")
	err = b.applySources(isoDir, b.spec.Image...)
	if err != nil {
		b.cfg.Logger.Errorf("Failed installing ISO image packages: %v", err)
		return err
	}

	err = b.prepareISORoot(isoDir, rootDir, uefiDir)
	if err != nil {
		b.cfg.Logger.Errorf("Failed preparing ISO's root tree: %v", err)
		return err
	}

	b.cfg.Logger.Infof("Creating ISO image...")
	err = b.burnISO(isoDir)
	if err != nil {
		b.cfg.Logger.Errorf("Failed creating ISO image: %v", err)
		return err
	}

	return err
}

func (b BuildISOAction) prepareISORoot(isoDir string, rootDir string, uefiDir string) error {
	kernel, initrd, err := b.e.FindKernelInitrd(rootDir)
	if err != nil {
		b.cfg.Logger.Error("Could not find kernel and/or initrd")
		return err
	}
	err = utils.MkdirAll(b.cfg.Fs, filepath.Join(isoDir, "boot"), constants.DirPerm)
	if err != nil {
		return err
	}
	//TODO document boot/kernel and boot/initrd expectation in bootloader config
	b.cfg.Logger.Debugf("Copying Kernel file %s to iso root tree", kernel)
	err = utils.CopyFile(b.cfg.Fs, kernel, filepath.Join(isoDir, constants.IsoKernelPath))
	if err != nil {
		return err
	}

	b.cfg.Logger.Debugf("Copying initrd file %s to iso root tree", initrd)
	err = utils.CopyFile(b.cfg.Fs, initrd, filepath.Join(isoDir, constants.IsoInitrdPath))
	if err != nil {
		return err
	}

	b.cfg.Logger.Info("Creating EFI image...")
	err = b.createEFI(rootDir, isoDir)
	if err != nil {
		return err
	}

	b.cfg.Logger.Info("Creating squashfs...")
	//err = utils.CreateSquashFS(b.cfg.Runner, b.cfg.Logger, rootDir, filepath.Join(isoDir, constants.IsoRootFile), constants.GetDefaultSquashfsOptions())
	err = CreateSquashFS(b.cfg, rootDir, filepath.Join(isoDir, constants.IsoRootFile))
	if err != nil {
		return err
	}

	return nil
}

func CreateSquashFS(cfg *types.BuildConfig, source string, destination string) (err error) {
	diskSize, err := utils.DirSize(cfg.Fs, source)
	if err != nil {
		return err
	}
	cfg.Logger.Logger.Info().Int64("size", diskSize).Msg("Calculated size")
	cfg.Logger.Logger.Info().Str("source", source).Str("destination", destination).Msg("Creating squashfs")

	mydisk, err := diskfs.Create(destination, diskSize, diskfs.Raw, 4096)
	if err != nil {
		return err
	}
	cfg.Logger.Logger.Info().Str("source", source).Str("destination", destination).Msg("Created squashfs")

	cfg.Logger.Logger.Info().Str("source", source).Str("destination", destination).Msg("Creating filesystem")
	squashFS, err := squashfs.Create(mydisk.File, mydisk.Size, 0, mydisk.LogicalBlocksize)
	if err != nil {
		return err
	}
	cfg.Logger.Logger.Info().Str("source", source).Str("destination", destination).Msg("Created filesystem")

	cfg.Logger.Logger.Info().Str("source", source).Str("destination", destination).Msg("Copying files")
	err = copyDir(cfg, source, source, squashFS)
	if err != nil {
		return err
	}
	cfg.Logger.Logger.Info().Str("source", source).Str("destination", destination).Msg("Copied files")

	err = squashFS.Finalize(squashfs.FinalizeOptions{})
	if err != nil {
		return err
	}
	return err
}

func copyDir(cfg *types.BuildConfig, root, src string, dst filesystem.FileSystem) error {
	// Get properties of source dir
	cfg.Logger.Logger.Debug().Str("path", src).Msg("Doing path")
	relPath, err := filepath.Rel(root, src)
	if err != nil {
		return fmt.Errorf("error getting relpath of %s: %w", src, err)
	}
	relPath = filepath.Join("/", relPath)
	cfg.Logger.Logger.Debug().Str("destination", relPath).Msg("Doing path")
	// Create the destination directory
	err = dst.Mkdir(relPath)
	if err != nil {
		return err
	}

	directory, _ := os.Open(src)
	objects, err := directory.Readdir(-1)
	if err != nil {
		return err
	}
	defer directory.Close()

	for _, obj := range objects {
		srcFilePath := filepath.Join(src, obj.Name())
		if obj.IsDir() {
			// Create sub-directories - recursively
			err = copyDir(cfg, root, srcFilePath, dst)
			if err != nil {
				cfg.Logger.Logger.Error().Err(err).Str("src", srcFilePath).Msg("Failed to copy directory")
				return err
			}
		} else {
			if obj.Mode()&os.ModeSymlink != 0 {
				// TODO: symlink
				continue
			} else {
				// Copy files
				err = copyFile(cfg, root, srcFilePath, dst)
				if err != nil {
					cfg.Logger.Logger.Error().Err(err).Str("src", srcFilePath).Msg("Failed to copy file")
					return err
				}
			}
		}
	}
	return nil
}

func copyFile(cfg *types.BuildConfig, root, src string, dst filesystem.FileSystem) error {
	cfg.Logger.Logger.Debug().Str("source", src).Msg("Copying file")
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	relPath, err := filepath.Rel(root, src)
	if err != nil {
		return fmt.Errorf("error getting relpath of %s: %w", src, err)
	}
	relPath = filepath.Join("/", relPath)

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	rw, err := dst.OpenFile(relPath, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return fmt.Errorf("error opening file %s: %w", src, err)
	}
	defer rw.Close()

	// Open the source file for reading

	// Copy the contents of the source file to the ISO file
	_, err = io.Copy(rw, source)
	if err != nil {
		return fmt.Errorf("error copying file: %w", err)
	}
	return err
}

// createEFI creates the EFI image that is used for booting
// it searches the rootfs for the shim/grub.efi file and copies it into a directory with the proper EFI structure
// then it generates a grub.cfg that chainloads into the grub.cfg of the livecd (which is the normal livecd grub config from luet packages)
// then it calculates the size of the EFI image based on the files copied and creates the image
func (b BuildISOAction) createEFI(rootdir string, isoDir string) error {
	var err error

	// rootfs /efi dir
	img := filepath.Join(isoDir, constants.IsoEFIPath)
	temp, _ := utils.TempDir(b.cfg.Fs, "", "enki-iso")
	err = utils.MkdirAll(b.cfg.Fs, filepath.Join(temp, constants.EfiBootPath), constants.DirPerm)
	if err != nil {
		b.cfg.Logger.Errorf("Failed creating temp efi dir: %v", err)
		return err
	}
	err = utils.MkdirAll(b.cfg.Fs, filepath.Join(isoDir, constants.EfiBootPath), constants.DirPerm)
	if err != nil {
		b.cfg.Logger.Errorf("Failed creating iso efi dir: %v", err)
		return err
	}

	err = b.copyShim(temp, rootdir)
	if err != nil {
		return err
	}

	err = b.copyGrub(temp, rootdir)
	if err != nil {
		return err
	}

	// Generate grub cfg that chainloads into the default livecd grub under /boot/grub2/grub.cfg
	// Its read from the root of the livecd, so we need to copy it into /EFI/BOOT/grub.cfg
	// This is due to the hybrid bios/efi boot mode of the livecd
	// the uefi.img is loaded into memory and run, but grub only sees the livecd root
	err = b.cfg.Fs.WriteFile(filepath.Join(isoDir, constants.EfiBootPath, constants.GrubCfg), []byte(constants.GrubEfiCfg), constants.FilePerm)
	if err != nil {
		b.cfg.Logger.Errorf("Failed writing grub.cfg: %v", err)
		return err
	}
	// Ubuntu efi searches for the grub.cfg file under /EFI/ubuntu/grub.cfg while we store it under /boot/grub2/grub.cfg
	// workaround this by copying it there as well
	// read the kairos-release from the rootfs to know if we are creating a ubuntu based iso
	var flavor string
	flavor, err = sdk.OSRelease("FLAVOR", filepath.Join(rootdir, "etc/kairos-release"))
	if err != nil {
		// fallback to os-release
		flavor, err = sdk.OSRelease("FLAVOR", filepath.Join(rootdir, "etc/os-release"))
		if err != nil {
			b.cfg.Logger.Warnf("Failed reading os-release from %s and %s: %v", filepath.Join(rootdir, "etc/kairos-release"), filepath.Join(rootdir, "etc/os-release"), err)
			return err
		}
	}
	b.cfg.Logger.Infof("Detected Flavor: %s", flavor)
	if strings.Contains(strings.ToLower(flavor), "ubuntu") {
		b.cfg.Logger.Infof("Ubuntu based ISO detected, copying grub.cfg to /EFI/ubuntu/grub.cfg")
		err = utils.MkdirAll(b.cfg.Fs, filepath.Join(isoDir, "EFI/ubuntu/"), constants.DirPerm)
		if err != nil {
			b.cfg.Logger.Errorf("Failed writing grub.cfg: %v", err)
			return err
		}
		err = b.cfg.Fs.WriteFile(filepath.Join(isoDir, "EFI/ubuntu/", constants.GrubCfg), []byte(constants.GrubEfiCfg), constants.FilePerm)
		if err != nil {
			b.cfg.Logger.Errorf("Failed writing grub.cfg: %v", err)
			return err
		}
	}

	// Calculate EFI image size based on artifacts
	efiSize, err := utils.DirSize(b.cfg.Fs, temp)
	if err != nil {
		return err
	}
	// align efiSize to the next 4MB slot
	align := int64(4 * 1024 * 1024)
	efiSizeMB := (efiSize/align*align + align) / (1024 * 1024)
	// Create the actual efi image
	err = b.e.CreateFileSystemImage(&v1.Image{
		File:  img,
		Size:  uint(efiSizeMB),
		FS:    constants.EfiFs,
		Label: constants.EfiLabel,
	})
	if err != nil {
		return err
	}
	b.cfg.Logger.Debugf("EFI image created at %s", img)
	// copy the files from the temporal efi dir into the EFI image
	files, err := b.cfg.Fs.ReadDir(temp)
	if err != nil {
		return err
	}

	for _, f := range files {
		// This copies the efi files into the efi img used for the boot
		b.cfg.Logger.Debugf("Copying %s to %s", filepath.Join(temp, f.Name()), img)
		_, err = b.cfg.Runner.Run("mcopy", "-s", "-i", img, filepath.Join(temp, f.Name()), "::")
		if err != nil {
			b.cfg.Logger.Errorf("Failed copying %s to %s: %v", filepath.Join(temp, f.Name()), img, err)
			return err
		}
	}

	return nil
}

// copyShim copies the shim files into the EFI partition
// tempdir is the temp dir where the EFI image is generated from
// rootdir is the rootfs where the shim files are searched for
func (b BuildISOAction) copyShim(tempdir, rootdir string) error {
	var fallBackShim string
	var err error
	// Get possible shim file paths
	shimFiles := sdk.GetEfiShimFiles(b.cfg.Arch)
	// Calculate shim path based on arch
	var shimDest string
	switch b.cfg.Arch {
	case constants.ArchAmd64, constants.Archx86:
		shimDest = filepath.Join(tempdir, constants.ShimEfiDest)
		fallBackShim = filepath.Join("/efi", constants.EfiBootPath, "bootx64.efi")
	case constants.ArchArm64:
		shimDest = filepath.Join(tempdir, constants.ShimEfiArmDest)
		fallBackShim = filepath.Join("/efi", constants.EfiBootPath, "bootaa64.efi")
	default:
		err = fmt.Errorf("not supported architecture: %v", b.cfg.Arch)
	}
	var shimDone bool
	for _, f := range shimFiles {
		_, err := b.cfg.Fs.Stat(filepath.Join(rootdir, f))
		if err != nil {
			b.cfg.Logger.Debugf("skip copying %s: not found", filepath.Join(rootdir, f))
			continue
		}
		b.cfg.Logger.Debugf("Copying %s to %s", filepath.Join(rootdir, f), shimDest)
		err = utils.CopyFile(
			b.cfg.Fs,
			filepath.Join(rootdir, f),
			shimDest,
		)
		if err != nil {
			b.cfg.Logger.Warnf("error reading %s: %s", filepath.Join(rootdir, f), err)
			continue
		}
		shimDone = true
		break
	}
	if !shimDone {
		// All failed...maybe we are on alpine which doesnt provide shim/grub.efi ?
		// In that case, we can just use the luet packaged artifacts
		err = utils.CopyFile(
			b.cfg.Fs,
			fallBackShim,
			shimDest,
		)
		if err != nil {
			b.cfg.Logger.Debugf("List of shim files searched for in %s: %s", rootdir, shimFiles)
			return fmt.Errorf("could not find any shim file to copy")
		}
		b.cfg.Logger.Debugf("Using fallback shim file %s", fallBackShim)
		// Also copy the shim.efi file into the rootfs so the installer can find it. Side effect of
		// alpine not providing shim/grub.efi and we not providing it from packages anymore
		_ = utils.MkdirAll(b.cfg.Fs, filepath.Join(rootdir, filepath.Dir(shimFiles[0])), constants.DirPerm)
		err = utils.CopyFile(
			b.cfg.Fs,
			fallBackShim,
			filepath.Join(rootdir, shimFiles[0]),
		)
		if err != nil {
			b.cfg.Logger.Debugf("Could not copy fallback shim into rootfs from %s to %s", fallBackShim, filepath.Join(rootdir, shimFiles[0]))
			return fmt.Errorf("could not copy fallback shim into rootfs from %s to %s", fallBackShim, filepath.Join(rootdir, shimFiles[0]))
		}
	}
	return err
}

// copyGrub copies the shim files into the EFI partition
// tempdir is the temp dir where the EFI image is generated from
// rootdir is the rootfs where the shim files are searched for
func (b BuildISOAction) copyGrub(tempdir, rootdir string) error {
	// this is shipped usually with osbuilder and the files come from livecd/grub2-efi-artifacts
	var fallBackGrub = filepath.Join("/efi", constants.EfiBootPath, "grub.efi")
	var err error
	// Get possible grub file paths
	grubFiles := sdk.GetEfiGrubFiles(b.cfg.Arch)
	var grubDone bool
	for _, f := range grubFiles {
		stat, err := b.cfg.Fs.Stat(filepath.Join(rootdir, f))
		if err != nil {
			b.cfg.Logger.Debugf("skip copying %s: not found", filepath.Join(rootdir, f))
			continue
		}
		// Same name as the source, shim looks for that name. We need to remove the .signed suffix
		nameDest := filepath.Join(tempdir, "EFI/BOOT", cleanupGrubName(stat.Name()))
		b.cfg.Logger.Debugf("Copying %s to %s", filepath.Join(rootdir, f), nameDest)

		err = utils.CopyFile(
			b.cfg.Fs,
			filepath.Join(rootdir, f),
			nameDest,
		)
		if err != nil {
			b.cfg.Logger.Warnf("error reading %s: %s", filepath.Join(rootdir, f), err)
			continue
		}
		grubDone = true
		break
	}
	if !grubDone {
		// All failed...maybe we are on alpine which doesnt provide shim/grub.efi ?
		// In that case, we can just use the luet packaged artifacts
		err = utils.CopyFile(
			b.cfg.Fs,
			fallBackGrub,
			filepath.Join(tempdir, "EFI/BOOT/grub.efi"),
		)
		if err != nil {
			b.cfg.Logger.Debugf("List of grub files searched for: %s", grubFiles)
			return fmt.Errorf("could not find any grub efi file to copy")
		}
		b.cfg.Logger.Debugf("Using fallback grub file %s", fallBackGrub)
		// Also copy the grub.efi file into the rootfs so the installer can find it. Side effect of
		// alpine not providing shim/grub.efi and we not providing it from packages anymore
		utils.MkdirAll(b.cfg.Fs, filepath.Join(rootdir, filepath.Dir(grubFiles[0])), constants.DirPerm)
		err = utils.CopyFile(
			b.cfg.Fs,
			fallBackGrub,
			filepath.Join(rootdir, grubFiles[0]),
		)
		if err != nil {
			b.cfg.Logger.Debugf("Could not copy fallback grub into rootfs from %s to %s", fallBackGrub, filepath.Join(rootdir, grubFiles[0]))
			return fmt.Errorf("could not copy fallback shim into rootfs from %s to %s", fallBackGrub, filepath.Join(rootdir, grubFiles[0]))
		}
	}
	return err
}

func (b BuildISOAction) burnISO(root string) error {
	cmd := "xorriso"
	var outputFile string
	var isoFileName string

	if b.cfg.Date {
		currTime := time.Now()
		isoFileName = fmt.Sprintf("%s.%s.iso", b.cfg.Name, currTime.Format("20060102"))
	} else {
		isoFileName = fmt.Sprintf("%s.iso", b.cfg.Name)
	}

	outputFile = isoFileName
	if b.cfg.OutDir != "" {
		outputFile = filepath.Join(b.cfg.OutDir, outputFile)
	}

	if exists, _ := utils.Exists(b.cfg.Fs, outputFile); exists {
		b.cfg.Logger.Warnf("Overwriting already existing %s", outputFile)
		err := b.cfg.Fs.Remove(outputFile)
		if err != nil {
			return err
		}
	}

	args := []string{
		"-volid", b.spec.Label, "-joliet", "on", "-padding", "0",
		"-outdev", outputFile, "-map", root, "/", "-chmod", "0755", "--",
	}
	args = append(args, constants.GetXorrisoBooloaderArgs(root)...)
	b.cfg.Logger.Logger.Info().Strs("args", args).Msg("running xorriso")
	out, err := b.cfg.Runner.Run(cmd, args...)
	b.cfg.Logger.Debugf("Xorriso: %s", string(out))
	if err != nil {
		b.cfg.Logger.Logger.Error().Err(err).Str("output", string(out)).Msg("Failed to build iso")
		return err
	}

	checksum, err := utils.CalcFileChecksum(b.cfg.Fs, outputFile)
	if err != nil {
		return fmt.Errorf("checksum computation failed: %w", err)
	}
	err = b.cfg.Fs.WriteFile(fmt.Sprintf("%s.sha256", outputFile), []byte(fmt.Sprintf("%s %s\n", checksum, isoFileName)), 0644)
	if err != nil {
		return fmt.Errorf("cannot write checksum file: %w", err)
	}

	return nil
}

func (b BuildISOAction) applySources(target string, sources ...*v1.ImageSource) error {
	for _, src := range sources {
		_, err := b.e.DumpSource(target, src)
		if err != nil {
			return err
		}
	}
	return nil
}

// cleanupGrubName will cleanup the grub name to provide a proper grub named file
// As the original name can contain several suffixes to indicate its signed status
// we need to clean them up before using them as the shim will look for a file with
// no suffixes
func cleanupGrubName(name string) string {
	// remove the .signed suffix if present
	clean := strings.TrimSuffix(name, ".signed")
	// remove the .dualsigned suffix if present
	clean = strings.TrimSuffix(clean, ".dualsigned")
	// remove the .signed.latest suffix if present
	clean = strings.TrimSuffix(clean, ".signed.latest")
	return clean
}
