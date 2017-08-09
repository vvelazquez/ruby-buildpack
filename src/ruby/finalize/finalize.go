package finalize

import (
	"fmt"
	"io/ioutil"
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

func (f *Finalizer) InstallPlugins() error {
	gem, err := f.Versions.HasGem("rails_12factor")
	if err != nil {
		return err
	}
	if gem {
		return nil
	}

	if err := f.installPluginStdoutLogger(); err != nil {
		return err
	}
	if err := f.installPluginServeStaticAssets(); err != nil {
		return err
	}
	return nil
}

func (f *Finalizer) installPluginStdoutLogger() error {
	gem, err := f.Versions.HasGem("rails_stdout_logging")
	if err != nil {
		return err
	}
	if gem {
		return nil
	}

	code := `
begin
  STDOUT.sync = true
  def Rails.cloudfoundry_stdout_logger
    logger = Logger.new(STDOUT)
    logger = ActiveSupport::TaggedLogging.new(logger) if defined?(ActiveSupport::TaggedLogging)
    level = ENV['LOG_LEVEL'].to_s.upcase
    level = 'INFO' unless %w[DEBUG INFO WARN ERROR FATAL UNKNOWN].include?(level)
    logger.level = Logger.const_get(level)
    logger
  end
  Rails.logger = Rails.application.config.logger = Rails.cloudfoundry_stdout_logger
rescue Exception => ex
  puts %Q{WARNING: Exception during rails_log_stdout init: #{ex.message}}
end
`

	if err := os.MkdirAll(filepath.Join(f.Stager.BuildDir(), "vendor", "plugins", "rails_log_stdout"), 0755); err != nil {
		return fmt.Errorf("Error creating rails_log_stdout plugin directory: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(f.Stager.BuildDir(), "vendor", "plugins", "rails_log_stdout", "init.rb"), []byte(code), 0644); err != nil {
		return fmt.Errorf("Error writing rails_log_stdout plugin file: %v", err)
	}
	return nil
}

func (f *Finalizer) installPluginServeStaticAssets() error {
	gem, err := f.Versions.HasGem("rails_serve_static_assets")
	if err != nil {
		return err
	}
	if gem {
		return nil
	}

	code := "Rails.application.class.config.serve_static_assets = true\n"

	if err := os.MkdirAll(filepath.Join(f.Stager.BuildDir(), "vendor", "plugins", "rails3_serve_static_assets"), 0755); err != nil {
		return fmt.Errorf("Error creating rails3_serve_static_assets plugin directory: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(f.Stager.BuildDir(), "vendor", "plugins", "rails3_serve_static_assets", "init.rb"), []byte(code), 0644); err != nil {
		return fmt.Errorf("Error writing rails3_serve_static_assets plugin file: %v", err)
	}
	return nil
}
