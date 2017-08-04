package finalize

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cloudfoundry/libbuildpack"
	"github.com/kr/text"
)

type Stager interface {
	BuildDir() string
}

type Finalizer struct {
	Stager   Stager
	Versions Versions
	Log      *libbuildpack.Logger
}

func Run(f *Finalizer) error {
	if err := f.PrecompileAssets(); err != nil {
		f.Log.Error("Error precompiling assets: %v", err)
	}

	data, err := f.GenerateReleaseYaml()
	if err != nil {
		f.Log.Error("Error generating release YAML: %v", err)
	}
	releasePath := filepath.Join(f.Stager.BuildDir(), "tmp", "ruby-buildpack-release-step.yml")
	libbuildpack.NewYAML().Write(releasePath, data)

	return nil
}

func (f *Finalizer) PrecompileAssets() error {
	cmd := exec.Command("bundle", "exec", "rake", "-n", "assets:precompile")
	cmd.Dir = f.Stager.BuildDir()
	if err := cmd.Run(); err != nil {
		return err
	}

	//TODO maybe care about user_env_hash ? w database_url mashed in (see ruby.rb:650)

	f.Log.BeginStep("Precompiling assets")
	startTime := time.Now()
	cmd = exec.Command("bundle", "exec", "rake", "assets:precompile")
	cmd.Dir = f.Stager.BuildDir()
	cmd.Stdout = text.NewIndentWriter(os.Stdout, []byte("       "))
	cmd.Stderr = text.NewIndentWriter(os.Stderr, []byte("       "))
	err := cmd.Run()

	f.Log.Info("Asset precompilation completed (%v)", time.Since(startTime))

	return err
}
