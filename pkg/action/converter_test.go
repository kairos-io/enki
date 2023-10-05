package action_test

import (
	"os"
	"path"

	v1mock "github.com/kairos-io/kairos-agent/v2/tests/mocks"

	. "github.com/kairos-io/enki/pkg/action"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConverterAction", func() {
	var rootfsPath, resultDir, imageName string
	var action *ConverterAction
	var runner *v1mock.FakeRunner
	var err error

	BeforeEach(func() {
		rootfsPath = prepareEmptyRootfs()
		resultDir, err = os.MkdirTemp("", "kairos-temp")
		Expect(err).ToNot(HaveOccurred())
		imageName = newImageName(10)
		runner = v1mock.NewFakeRunner()
		action = NewConverterAction(rootfsPath, path.Join(resultDir, "image.tar"), imageName, "quay.io/kairos/framework:master_ubuntu", runner)
	})

	AfterEach(func() {
		cleanupDir(rootfsPath)
		cleanupDir(resultDir)
		removeImage(imageName)
	})

	// TODO: Move to e2e tests
	It("adds the framework bits", func() {
		// TODO: Run enki next to kaniko (in an image?)
		// CGO_ENABLED=0 go build -ldflags '-extldflags "-static"' -o build/enki && docker run -it -e PATH=/kaniko -v /tmp -v /home/dimitris/workspace/kairos/osbuilder/tmp/rootfs/:/context -v "$PWD/build/enki":/enki -v $PWD:/build --rm --entrypoint "/enki" gcr.io/kaniko-project/executor:latest convert /context

		//loadImage(fmt.Sprintf("%s/image.tar", resultDir))
	})

	It("runs the kaniko executor", func() {
		Expect(action.Run()).ToNot(HaveOccurred())
		Expect(runner.IncludesCmds([][]string{
			{"executor"},
		})).To(BeNil())
	})
})
