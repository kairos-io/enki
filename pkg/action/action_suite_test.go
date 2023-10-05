/*
Copyright © 2022 SUSE LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package action_test

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestActionSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Actions test suite")
}

func prepareEmptyRootfs() string {
	dir, err := os.MkdirTemp("", "kairos-temp")
	Expect(err).ToNot(HaveOccurred())

	return dir
}
func prepareRootfsFromImage(imageURI string) string {
	dir, err := os.MkdirTemp("", "kairos-temp")
	Expect(err).ToNot(HaveOccurred())

	cmd := exec.Command("/bin/sh", "-c",
		fmt.Sprintf("docker run -v %s:/work quay.io/luet/base util unpack %s /work", dir, imageURI))
	out, err := cmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(out))

	return dir
}

// Cleanup in docker to use the same permissions as those when we created.
// This way we avoid sudo.
func cleanupDir(path string) {
	cmd := exec.Command("/bin/sh", "-c",
		fmt.Sprintf("docker run --rm -v %[1]s:/work busybox /bin/sh -c 'rm -rf /work/*'", path))
	out, err := cmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), string(out))
	Expect(os.RemoveAll(path)).ToNot(HaveOccurred())
}

func removeImage(image string) {
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("docker rmi %s:latest", image))
	_ = cmd.Run() // Best effort, image may not be there if something failed.
}

func loadImage(imageTarPath string) {
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("cat %s | docker load", imageTarPath))
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
