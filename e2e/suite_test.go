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

type Enki struct {
	Path           string
	ContainerImage string
	Dirs           []string // directories to mount from host
}

func TestEnkiE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Enki end to end test suite")
}

func NewEnki(image string, dirs ...string) *Enki {
	tmpDir, err := os.MkdirTemp("", "enki-e2e-tmp")
	Expect(err).ToNot(HaveOccurred())
	enkiBinary := path.Join(tmpDir, "enki")

	compileEnki(enkiBinary)

	return &Enki{ContainerImage: image, Path: enkiBinary, Dirs: dirs}
}

// enki relies on various external binaries. To make sure those dependencies
// are in place (or to test the behavior of enki when they are not), we run enki
// in a container using this function.
func (e *Enki) Run(enkiArgs ...string) (string, error) {
	return e.ContainerRun("/bin/enki", enkiArgs...)
}

func (e *Enki) ContainerRun(entrypoint string, args ...string) (string, error) {
	dockerArgs := []string{
		"run", "--rm",
		"--entrypoint", entrypoint,
		"-v", fmt.Sprintf("%s:/bin/enki", e.Path),
	}

	for _, d := range e.Dirs {
		dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%[1]s:%[1]s", d))
	}

	dockerArgs = append(dockerArgs, e.ContainerImage)
	dockerArgs = append(dockerArgs, args...)

	cmd := exec.Command("docker", dockerArgs...)

	out, err := cmd.CombinedOutput()

	return string(out), err
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
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = rootDir

	out, err := cmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(out))
}
