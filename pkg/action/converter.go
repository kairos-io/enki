package action

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
)

// ConverterAction is the action that converts a non-kairos image to a Kairos one.
// The conversion happens in a best-effort manner. It's not guaranteed that
// any distribution will successfully be converted to a Kairos flavor. See
// the Kairos releases for known-to-work flavors.
// The "input" of this action is a directory where the rootfs is extracted.
// [TBD] The output is the same directory updated to be a Kairos image
type ConverterAction struct {
	rootFSPath string
	resultPath string
	imageName  string
	Runner     Runner
}

// A runner that can shell-out to other commands but also be mocked in tests.
type Runner interface {
	Run(string, ...string) ([]byte, error)
}

type RealRunner struct {
	Logger v1.Logger
}

func (r RealRunner) Run(command string, args ...string) ([]byte, error) {
	cmd := exec.Command(command, args...)

	return cmd.CombinedOutput()
}

func NewConverterAction(rootfsPath, resultPath, imageName string, runner Runner) *ConverterAction {
	return &ConverterAction{
		rootFSPath: rootfsPath,
		resultPath: resultPath,
		imageName:  imageName,
		Runner:     runner,
	}
}

// https://github.com/GoogleContainerTools/kaniko/issues/1007
// docker run -it -v $PWD:/work:rw  --rm gcr.io/kaniko-project/executor:latest --dockerfile /work/Dockerfile --context dir:///work/rootfs --destination whatever --tar-path /work/image.tar --no-push

// Run assumes the `kaniko` executable is in PATH as it shells out to it.
// The best way to do that is to spin up a container with the upstream
// image (gcr.io/kaniko-project/executor:latest) and mount enki in it.
// E.g.
// CGO_ENABLED=0 go build -ldflags '-extldflags "-static"' -o build/enki && docker run -it -e PATH=/kaniko -v /tmp -v /home/dimitris/workspace/kairos/osbuilder/tmp/rootfs/:/context -v "$PWD/build/enki":/enki -v $PWD:/build --rm --entrypoint "/enki" gcr.io/kaniko-project/executor:latest convert /context
func (ca *ConverterAction) Run() (err error) {
	da := NewDockerfileAction(ca.rootFSPath, "")
	dockerfile, err := da.Run()
	if err != nil {
		return err
	}
	defer os.Remove(dockerfile)

	err = ca.addDockerIgnore()
	if err != nil {
		return
	}
	defer ca.removeDockerIgnore()

	out, err := ca.BuildWithKaniko(dockerfile, ca.resultPath)
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}

	return
}

// https://github.com/GoogleContainerTools/kaniko/issues/1007
// Create a .dockerignore in the rootfs directory to skip these:
// https://github.com/GoogleContainerTools/kaniko/pull/1724/files#diff-1e90758e2fb0f26bdbfe7a40aafc4b4796cbf808842703e52e16c1f36b8da7dcR89
func (ca *ConverterAction) addDockerIgnore() error {
	content := `
/dev
/proc
/run
/sys
/var/run
`

	f, err := os.Create(path.Join(ca.rootFSPath, ".dockerignore"))
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write([]byte(content)); err != nil {
		return err
	}

	return nil
}

func (ca *ConverterAction) removeDockerIgnore() error {
	return os.RemoveAll(path.Join(ca.rootFSPath, ".dockerignore"))
}

func (ca *ConverterAction) BuildWithKaniko(dockerfile, resultPath string) (string, error) {
	d, err := ca.Runner.Run(
		"executor",
		//"--verbosity", "debug",
		"--dockerfile", dockerfile,
		"--context", ca.rootFSPath,
		"--destination", ca.imageName, // This is the name of the image when you: cat image.tar | docker load
		"--tar-path", resultPath, // TODO: Do we want this extracted to the rootFSPath?
		"--no-push",
	)

	return string(d), err
}
