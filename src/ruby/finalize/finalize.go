package finalize

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cloudfoundry/libbuildpack"
	"github.com/kr/text"
)

// type Manifest interface {
// 	RootDir() string
// }

type Stager interface {
	BuildDir() string
	// DepDir() string
}

type Finalizer struct {
	Stager Stager
	Log    *libbuildpack.Logger
	// Logfile     *os.File
	// Manifest    Manifest
	// StartScript string
}

func Run(f *Finalizer) error {
	if err := f.PrecompileAssets(); err != nil {
		f.Log.Error("Error precompiling assets: %v", err)
	}

	if err := f.GenerateReleaseYaml(); err != nil {
		f.Log.Error("Error generating release YAML: %v", err)
	}

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

func (f *Finalizer) GenerateReleaseYaml() error {
	if err := os.MkdirAll(filepath.Join(f.Stager.BuildDir(), "tmp"), 0755); err != nil {
		return err
	}

	releasePath := filepath.Join(f.Stager.BuildDir(), "tmp", "ruby-buildpack-release-step.yml")
	return ioutil.WriteFile(releasePath, []byte(`---
config_vars:
  LANG: en_US.UTF-8
  RAILS_ENV: production
  RACK_ENV: production
  SECRET_KEY_BASE: 1234
  RAILS_SERVE_STATIC_FILES: enabled
  RAILS_LOG_TO_STDOUT: enabled
default_process_types:
  rake: bundle exec rake
  console: bin/rails console
  web: bin/rails server -b 0.0.0.0 -p $PORT -e $RAILS_ENV
  worker: bundle exec rake jobs:work
`), 0755)
}
