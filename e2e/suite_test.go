package e2e_test

import (
	"math/rand"
	"os"
	"os/exec"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestE2ESuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Enki end to end test suite")
}

// enki is a helper function to act as if we built the enki binary and run it
func enki(params ...string) string {
	cmd := exec.Command("go", append([]string{"run", "../main.go"}, params...)...)
	out, err := cmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(out))

	return string(out)
}

func generateDockerfile(baseImage, frameworkImage, osReleaseVarsPath string) string {
	out := enki("dockerfile",
		"--base-image-uri", baseImage,
		"--framework-image", frameworkImage,
		"--os-release-vars-path", osReleaseVarsPath,
	)

	file, err := os.CreateTemp("", "kairos-dockerfile-")
	Expect(err).ToNot(HaveOccurred())
	_, err = file.Write([]byte(out))
	Expect(err).ToNot(HaveOccurred())

	return file.Name()
}

func buildDockerfile(dockerfilePath string) string {
	// TODO: Run `docker build` on the provided dockerfile and generate a randomly
	// named image. Return the name of the image.

	dir, err := os.MkdirTemp("", "kairos-test-context-")
	Expect(err).ToNot(HaveOccurred())

	imageName := newImageName(10)

	cmd := exec.Command("docker", "build", "-f", dockerfilePath, "-t", imageName, dir)
	out, err := cmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(out))

	return imageName
}

func deleteImage(imageName string) {
	cmd := exec.Command("docker", "rmi", imageName)
	out, err := cmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(out))
}

func newImageName(n int) string {
	rand.Seed(time.Now().UnixNano())
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
