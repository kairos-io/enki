package action

import (
	"fmt"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
	"path/filepath"
	"time"

	"github.com/kairos-io/enki/pkg/constants"
	"github.com/kairos-io/enki/pkg/types"
	"github.com/kairos-io/enki/pkg/utils"
	"github.com/kairos-io/kairos-agent/v2/pkg/elemental"
	sdk "github.com/kairos-io/kairos-sdk/utils"
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
		b.cfg.Logger.Errorf("Failed preparing ISO's root tree: %v", err)
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

	b.cfg.Logger.Info("Creating squashfs...")
	err = utils.CreateSquashFS(b.cfg.Runner, b.cfg.Logger, rootDir, filepath.Join(isoDir, constants.IsoRootFile), constants.GetDefaultSquashfsOptions())
	if err != nil {
		return err
	}

	b.cfg.Logger.Info("Creating EFI image...")
	err = b.createEFI(rootDir, isoDir)
	if err != nil {
		return err
	}
	return nil
}

func (b BuildISOAction) createEFI(rootdir string, isoDir string) error {
	var err error
	var fallBackShim string
	var fallBackGrub = filepath.Join(rootdir, "/efi", "grub.efi")
	img := filepath.Join(isoDir, constants.IsoEFIPath)
	temp, _ := utils.TempDir(b.cfg.Fs, "", "enki-iso")
	utils.MkdirAll(b.cfg.Fs, filepath.Join(temp, constants.EfiBootPath), constants.DirPerm)
	utils.MkdirAll(b.cfg.Fs, filepath.Join(isoDir, constants.EfiBootPath), constants.DirPerm)
	// Get possible shim file paths
	shimFiles := sdk.GetEfiShimFiles(b.cfg.Arch)
	// Get possible grub file paths
	grubFiles := sdk.GetEfiGrubFiles(b.cfg.Arch)

	// Calculate shim path based on arch
	var shimDest string
	switch b.cfg.Arch {
	case constants.ArchAmd64, constants.Archx86:
		shimDest = filepath.Join(temp, constants.ShimEfiDest)
		fallBackShim = filepath.Join(rootdir, "/efi", "bootx64.efi")
	case constants.ArchArm64:
		shimDest = filepath.Join(temp, constants.ShimEfiArmDest)
		fallBackShim = filepath.Join(rootdir, "/efi", "bootaa64.efi")
	default:
		err = fmt.Errorf("not supported architecture: %v", b.cfg.Arch)
	}

	shimDone := false
	for _, f := range shimFiles {
		_, err := b.cfg.Fs.Stat(filepath.Join(rootdir, f))
		if err != nil {
			b.cfg.Logger.Warnf("skip copying %s: %s", filepath.Join(rootdir, f), err)
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
	}

	grubDone := false
	for _, f := range grubFiles {
		stat, err := b.cfg.Fs.Stat(filepath.Join(rootdir, f))
		if err != nil {
			b.cfg.Logger.Warnf("skip copying %s: %s", filepath.Join(rootdir, f), err)
			continue
		}
		// Same name as the source, shim looks for that name.
		nameDest := filepath.Join(temp, "EFI/BOOT", stat.Name())
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
			filepath.Join(temp, "EFI/BOOT/grub.efi"),
		)
		if err != nil {
			b.cfg.Logger.Debugf("List of grub files searched for: %s", grubFiles)
			return fmt.Errorf("could not find any grub efi file to copy")
		}
		b.cfg.Logger.Debugf("Using fallback grub file %s", fallBackGrub)
	}

	// Generate grub cfg that chainloads into the default livecd grub under /boot/grub2/grub.cfg
	// Its read from the root of the livecd, so we need to copy it into /EFI/BOOT/grub.cfg
	// This is due to the hybrid bios/efi boot mode of the livecd
	// the uefi.img is loaded into memory and run, but grub only sees the livecd root
	b.cfg.Fs.WriteFile(filepath.Join(isoDir, constants.EfiBootPath, constants.GrubCfg), []byte(constants.GrubEfiCfg), constants.FilePerm)

	// Calculate EFI image size based on artifacts
	efiSize, err := utils.DirSize(b.cfg.Fs, temp)
	if err != nil {
		return err
	}
	// align efiSize to the next 4MB slot
	align := int64(4 * 1024 * 1024)
	efiSizeMB := (efiSize/align*align + align) / (1024 * 1024)

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

	files, err := b.cfg.Fs.ReadDir(temp)
	if err != nil {
		return err
	}

	for _, f := range files {
		// This copies the efi files into the efi img used for the boot
		b.cfg.Logger.Debugf("Copying %s to %s", filepath.Join(temp, f.Name()), img)
		_, err = b.cfg.Runner.Run("mcopy", "-s", "-i", img, filepath.Join(temp, f.Name()), "::")
		if err != nil {
			return err
		}
	}

	return nil
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

	out, err := b.cfg.Runner.Run(cmd, args...)
	b.cfg.Logger.Debugf("Xorriso: %s", string(out))
	if err != nil {
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
