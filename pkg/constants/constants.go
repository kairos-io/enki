package constants

import (
	"fmt"
	"os"
	"path/filepath"
)

type UkiOutput string

const IsoOutput UkiOutput = "iso"
const ContainerOutput UkiOutput = "container"
const DefaultOutput UkiOutput = "uki"

func OutPutTypes() []string {
	return []string{string(IsoOutput), string(ContainerOutput), string(DefaultOutput)}
}

const (
	GrubDefEntry   = "Kairos"
	EfiLabel       = "COS_GRUB"
	ISOLabel       = "COS_LIVE"
	MountBinary    = "/usr/bin/mount"
	EfiFs          = "vfat"
	IsoRootFile    = "rootfs.squashfs"
	IsoEFIPath     = "/boot/uefi.img"
	BuildImgName   = "elemental"
	EfiBootPath    = "/EFI/BOOT"
	ShimEfiDest    = EfiBootPath + "/bootx64.efi"
	ShimEfiArmDest = EfiBootPath + "/bootaa64.efi"
	GrubCfg        = "grub.cfg"
	GrubPrefixDir  = "/boot/grub2"
	GrubEfiCfg     = "search --no-floppy --file --set=root " + IsoKernelPath +
		"\nset prefix=($root)" + GrubPrefixDir +
		"\nconfigfile $prefix/" + GrubCfg

	IsoHybridMBR   = "/boot/x86_64/loader/boot_hybrid.img"
	IsoBootCatalog = "/boot/x86_64/boot.catalog"
	IsoBootFile    = "/boot/x86_64/loader/eltorito.img"

	// These paths are arbitrary but coupled to grub.cfg
	IsoKernelPath = "/boot/kernel"
	IsoInitrdPath = "/boot/initrd"

	// Default directory and file fileModes
	DirPerm        = os.ModeDir | os.ModePerm
	FilePerm       = 0666
	NoWriteDirPerm = 0555 | os.ModeDir
	TempDirPerm    = os.ModePerm | os.ModeSticky | os.ModeDir

	ArchAmd64   = "amd64"
	Archx86     = "x86_64"
	ArchArm64   = "arm64"
	Archaarch64 = "aarch64"

	UkiCmdline            = "console=ttyS0 console=tty1 net.ifnames=1 rd.immucore.oemlabel=COS_OEM rd.immucore.debug rd.immucore.oemtimeout=2 rd.immucore.uki selinux=0"
	UkiCmdlineInstall     = "install-mode"
	UkiSystemdBootx86     = "/usr/kairos/systemd-bootx64.efi"
	UkiSystemdBootStubx86 = "/usr/kairos/linuxx64.efi.stub"
	UkiSystemdBootArm     = "/usr/kairos/systemd-bootaa64.efi"
	UkiSystemdBootStubArm = "/usr/kairos/linuxaa64.efi.stub"

	EfiFallbackNamex86 = "BOOTX64.EFI"
	EfiFallbackNameArm = "BOOTAA64.EFI"

	ArtifactBaseName = "artifact"
)

// GetDefaultSquashfsOptions returns the default options to use when creating a squashfs
func GetDefaultSquashfsOptions() []string {
	return []string{"-b", "1024k"}
}

func GetXorrisoBooloaderArgs(root string) []string {
	args := []string{
		"-boot_image", "grub", fmt.Sprintf("bin_path=%s", IsoBootFile),
		"-boot_image", "grub", fmt.Sprintf("grub2_mbr=%s/%s", root, IsoHybridMBR),
		"-boot_image", "grub", "grub2_boot_info=on",
		"-boot_image", "any", "partition_offset=16",
		"-boot_image", "any", fmt.Sprintf("cat_path=%s", IsoBootCatalog),
		"-boot_image", "any", "cat_hidden=on",
		"-boot_image", "any", "boot_info_table=on",
		"-boot_image", "any", "platform_id=0x00",
		"-boot_image", "any", "emul_type=no_emulation",
		"-boot_image", "any", "load_size=2048",
		"-append_partition", "2", "0xef", filepath.Join(root, IsoEFIPath),
		"-boot_image", "any", "next",
		"-boot_image", "any", "efi_path=--interval:appended_partition_2:all::",
		"-boot_image", "any", "platform_id=0xef",
		"-boot_image", "any", "emul_type=no_emulation",
	}
	return args
}
