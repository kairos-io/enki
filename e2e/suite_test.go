package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const DefaultRunImage = "busybox"

type Enki struct {
	Path           string
	ContainerImage string
}

func TestEnkiE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Enki end to end test suite")
}

func NewEnki(image ...string) *Enki {
	var runImage string

	if len(image) == 0 {
		runImage = DefaultRunImage
	} else {
		runImage = image[0]
	}

	tmpDir, err := os.MkdirTemp("", "enki-e2e-tmp")
	Expect(err).ToNot(HaveOccurred())
	enkiBinary := path.Join(tmpDir, "enki")

	compileEnki(enkiBinary)

	return &Enki{ContainerImage: runImage, Path: enkiBinary}
}

// enki relies on various external binaries. To make sure those dependencies
// are in place (or to test the behavior of enki when they are not), we run enki
// in a container using this function.
func (e *Enki) Run(args ...string) string {
	cmd := exec.Command("docker",
		append([]string{
			"run", "--rm",
			"--entrypoint", "/bin/enki",
			"-v", fmt.Sprintf("%s:/bin/enki", e.Path),
			e.ContainerImage}, args...)...)

	out, err := cmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(out))

	return string(out)
}

func (e *Enki) Cleanup() {
	dir := filepath.Dir(e.Path)
	Expect(os.RemoveAll(dir)).ToNot(HaveOccurred())
}

func compileEnki(targetPath string) {
	testDir, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())

	parentDir := path.Join(testDir, "..")
	rootDir, err := filepath.Abs(parentDir)
	Expect(err).ToNot(HaveOccurred())

	cmd := exec.Command("go", "build", "-o", targetPath)
	cmd.Dir = rootDir

	out, err := cmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(out))
}
