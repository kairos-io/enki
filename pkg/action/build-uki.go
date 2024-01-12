package action

import (
	"os"
	"os/exec"

	"github.com/kairos-io/enki/pkg/types"
	"github.com/kairos-io/kairos-agent/v2/pkg/elemental"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
)

type BuildUKIAction struct {
	img     *v1.ImageSource
	e       *elemental.Elemental
	ukiFile string
}

func NewBuildUKIAction(cfg *types.BuildConfig, img *v1.ImageSource, result string) *BuildUKIAction {
	b := &BuildUKIAction{
		img:     img,
		e:       elemental.NewElemental(&cfg.Config),
		ukiFile: result,
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
		"ukify",
	}

	for _, b := range neededBinaries {
		_, err := exec.LookPath(b)
		if err != nil {
			return err
		}
	}

	return nil
}
