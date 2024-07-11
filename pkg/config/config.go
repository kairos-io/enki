package config

import (
	"reflect"
	"runtime"

	"github.com/kairos-io/enki/internal/version"
	"github.com/kairos-io/enki/pkg/constants"
	"github.com/kairos-io/enki/pkg/types"
	"github.com/kairos-io/enki/pkg/utils"
	"github.com/kairos-io/kairos-agent/v2/pkg/cloudinit"
	"github.com/kairos-io/kairos-agent/v2/pkg/config"
	"github.com/kairos-io/kairos-agent/v2/pkg/http"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
	sdkTypes "github.com/kairos-io/kairos-sdk/types"
	"github.com/mitchellh/mapstructure"
	"github.com/sanity-io/litter"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/twpayne/go-vfs/v4"
)

var decodeHook = viper.DecodeHook(
	mapstructure.ComposeDecodeHookFunc(
		UnmarshalerHook(),
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	),
)

func WithFs(fs v1.FS) func(r *config.Config) error {
	return func(r *config.Config) error {
		r.Fs = fs
		return nil
	}
}

func WithLogger(logger sdkTypes.KairosLogger) func(r *config.Config) error {
	return func(r *config.Config) error {
		r.Logger = logger
		return nil
	}
}

func WithSyscall(syscall v1.SyscallInterface) func(r *config.Config) error {
	return func(r *config.Config) error {
		r.Syscall = syscall
		return nil
	}
}

func WithRunner(runner v1.Runner) func(r *config.Config) error {
	return func(r *config.Config) error {
		r.Runner = runner
		return nil
	}
}

func WithClient(client v1.HTTPClient) func(r *config.Config) error {
	return func(r *config.Config) error {
		r.Client = client
		return nil
	}
}

func WithCloudInitRunner(ci v1.CloudInitRunner) func(r *config.Config) error {
	return func(r *config.Config) error {
		r.CloudInitRunner = ci
		return nil
	}
}

func WithArch(arch string) func(r *config.Config) error {
	return func(r *config.Config) error {
		r.Arch = arch
		return nil
	}
}

func WithImageExtractor(extractor v1.ImageExtractor) func(r *config.Config) error {
	return func(r *config.Config) error {
		r.ImageExtractor = extractor
		return nil
	}
}

type GenericOptions func(a *config.Config) error

func ReadConfigBuild(configDir string, flags *pflag.FlagSet) (*types.BuildConfig, error) {
	var logLevel string
	if viper.GetBool("debug") {
		logLevel = "debug"
	} else {
		logLevel = "info"
	}
	logger := sdkTypes.NewKairosLogger("enki", logLevel, viper.GetBool("quiet"))
	logger.Infof("Starting enki version %s", version.GetVersion())

	if configDir == "" {
		configDir = "."
	}

	// TODO: Why didn't we set an ImageExtractor?
	// How is build-iso used? What does it set it to and where? (the viper config below?)
	cfg := NewBuildConfig(
		WithImageExtractor(v1.OCIImageExtractor{}),
		WithLogger(logger),
	)

	viper.AddConfigPath(configDir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("manifest.yaml")
	// If a config file is found, read it in.
	_ = viper.MergeInConfig()

	// Bind buildconfig flags
	bindGivenFlags(viper.GetViper(), flags)

	// unmarshal all the vars into the config object
	err := viper.Unmarshal(cfg, setDecoder, decodeHook)
	if err != nil {
		cfg.Logger.Warnf("error unmarshalling config: %s", err)
	}

	err = cfg.Sanitize()
	cfg.Logger.Debugf("Full config loaded: %s", litter.Sdump(cfg))
	return cfg, err
}

func ReadBuildISO(b *types.BuildConfig, flags *pflag.FlagSet) (*types.LiveISO, error) {
	iso := NewISO()
	vp := viper.Sub("iso")
	if vp == nil {
		vp = viper.New()
	}
	// Bind build-iso cmd flags
	bindGivenFlags(vp, flags)

	err := vp.Unmarshal(iso, setDecoder, decodeHook)
	if err != nil {
		b.Logger.Warnf("error unmarshalling LiveISO: %s", err)
	}
	err = iso.Sanitize()
	b.Logger.Debugf("Loaded LiveISO: %s", litter.Sdump(iso))
	return iso, err
}

func NewISO() *types.LiveISO {
	return &types.LiveISO{
		Label:     constants.ISOLabel,
		GrubEntry: constants.GrubDefEntry,
		UEFI:      []*v1.ImageSource{},
		Image:     []*v1.ImageSource{},
	}
}

func NewBuildConfig(opts ...GenericOptions) *types.BuildConfig {
	b := &types.BuildConfig{
		Config: *NewConfig(opts...),
		Name:   constants.BuildImgName,
	}
	return b
}

func NewConfig(opts ...GenericOptions) *config.Config {
	log := sdkTypes.NewKairosLogger("enki", "info", false)
	arch, err := utils.GolangArchToArch(runtime.GOARCH)
	if err != nil {
		log.Errorf("invalid arch: %s", err.Error())
		return nil
	}

	c := &config.Config{
		Fs:                    vfs.OSFS,
		Logger:                log,
		Syscall:               &v1.RealSyscall{},
		Client:                http.NewClient(),
		Arch:                  arch,
		SquashFsNoCompression: true,
	}
	for _, o := range opts {
		err := o(c)
		if err != nil {
			log.Errorf("error applying config option: %s", err.Error())
			return nil
		}
	}

	// delay runner creation after we have run over the options in case we use WithRunner
	if c.Runner == nil {
		c.Runner = &v1.RealRunner{Logger: &c.Logger}
	}

	// Now check if the runner has a logger inside, otherwise point our logger into it
	// This can happen if we set the WithRunner option as that doesn't set a logger
	if c.Runner.GetLogger() == nil {
		c.Runner.SetLogger(&c.Logger)
	}

	// Delay the yip runner creation, so we set the proper logger instead of blindly setting it to the logger we create
	// at the start of NewRunConfig, as WithLogger can be passed on init, and that would result in 2 different logger
	// instances, on the config.Logger and the other on config.CloudInitRunner
	if c.CloudInitRunner == nil {
		c.CloudInitRunner = cloudinit.NewYipCloudInitRunner(c.Logger, c.Runner, vfs.OSFS)
	}
	litter.Config.HidePrivateFields = false

	return c
}

// BindGivenFlags binds to viper only passed flags, ignoring any non provided flag
func bindGivenFlags(vp *viper.Viper, flagSet *pflag.FlagSet) {
	if flagSet != nil {
		flagSet.VisitAll(func(f *pflag.Flag) {
			if f.Changed {
				_ = vp.BindPFlag(f.Name, f)
			}
		})
	}
}

// setDecoder sets ZeroFields mastructure attribute to true
func setDecoder(config *mapstructure.DecoderConfig) {
	// Make sure we zero fields before applying them, this is relevant for slices
	// so we do not merge with any already present value and directly apply whatever
	// we got form configs.
	config.ZeroFields = true
}

type Unmarshaler interface {
	CustomUnmarshal(interface{}) (bool, error)
}

func UnmarshalerHook() mapstructure.DecodeHookFunc {
	return func(from reflect.Value, to reflect.Value) (interface{}, error) {
		// get the destination object address if it is not passed by reference
		if to.CanAddr() {
			to = to.Addr()
		}
		// If the destination implements the unmarshaling interface
		u, ok := to.Interface().(Unmarshaler)
		if !ok {
			return from.Interface(), nil
		}
		// If it is nil and a pointer, create and assign the target value first
		if to.IsNil() && to.Type().Kind() == reflect.Ptr {
			to.Set(reflect.New(to.Type().Elem()))
			u = to.Interface().(Unmarshaler)
		}
		// Call the custom unmarshaling method
		cont, err := u.CustomUnmarshal(from.Interface())
		if cont {
			// Continue with the decoding stack
			return from.Interface(), err
		}
		// Decoding finalized
		return to.Interface(), err
	}
}
