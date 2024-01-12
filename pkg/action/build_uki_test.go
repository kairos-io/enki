package action_test

import (
	"os"
	"path"

	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"

	. "github.com/kairos-io/enki/pkg/action"
	"github.com/kairos-io/enki/pkg/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("BuildUKIAction", func() {
	var action *BuildUKIAction
	var resultDir string
	var resultFile string
	var err error

	imgSource, err := v1.NewSrcFromURI("busybox") // TODO: Change to a fixture or an actual OS image?
	Expect(err).ToNot(HaveOccurred())

	BeforeEach(func() {
		resultDir, err = os.MkdirTemp("", "enki-build-uki-test-")
		Expect(err).ToNot(HaveOccurred())
		resultFile = path.Join(resultDir, "result.uki")

		cfg := config.NewBuildConfig(
			config.WithImageExtractor(v1.OCIImageExtractor{}),
		)

		action = NewBuildUKIAction(cfg, imgSource, resultFile)
	})

	AfterEach(func() {
		os.RemoveAll(resultDir)
	})

	It("successfully builds an UKI from a Docker image", func() {
		err := action.Run()
		Expect(err).ToNot(HaveOccurred())

		_, err = os.Stat(resultFile)
		Expect(err).ToNot(HaveOccurred())
	})
})
