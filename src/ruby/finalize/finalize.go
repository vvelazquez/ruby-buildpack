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
	Stager           Stager
	Versions         Versions
	Log              *libbuildpack.Logger
	Gem12Factor      bool
	GemStaticAssets  bool
	GemStdoutLogging bool
	RailsVersion     int
}

func Run(f *Finalizer) error {
	f.Log.BeginStep("Finalizing Ruby")

	if err := f.Setup(); err != nil {
		f.Log.Error("Error determining versions: %v", err)
	}

	if err := f.WriteDatabaseYml(); err != nil {
		f.Log.Error("Error writing database.yml: %v", err)
	}

	if err := f.PrecompileAssets(); err != nil {
		f.Log.Error("Error precompiling assets: %v", err)
	}

	f.BestPracticeWarnings()

	data, err := f.GenerateReleaseYaml()
	if err != nil {
		f.Log.Error("Error generating release YAML: %v", err)
	}
	releasePath := filepath.Join(f.Stager.BuildDir(), "tmp", "ruby-buildpack-release-step.yml")
	libbuildpack.NewYAML().Write(releasePath, data)

	return nil
}

func (f *Finalizer) Setup() error {
	var err error

	f.Gem12Factor, err = f.Versions.HasGem("rails_12factor")
	if err != nil {
		return err
	}

	f.GemStdoutLogging, err = f.Versions.HasGem("rails_stdout_logging")
	if err != nil {
		return err
	}

	f.GemStaticAssets, err = f.Versions.HasGem("rails_serve_static_assets")
	if err != nil {
		return err
	}

	f.RailsVersion, err = f.Versions.GemMajorVersion("rails")
	if err != nil {
		return err
	}

	return nil
}

func (f *Finalizer) WriteDatabaseYml() error {
	if exists, err := libbuildpack.FileExists(filepath.Join(f.Stager.BuildDir(), "config")); err != nil {
		return err
	} else if !exists {
		return nil
	}
	if rails41Plus, err := f.Versions.HasGemVersion("activerecord", ">=4.1.0.beta"); err != nil {
		return err
	} else if rails41Plus {
		return nil
	}

	f.Log.BeginStep("Writing config/database.yml to read from DATABASE_URL")
	if err := ioutil.WriteFile(filepath.Join(f.Stager.BuildDir(), "config", "database.yml"), []byte(config_database_yml), 0644); err != nil {
		return err
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

	if f.RailsVersion >= 4 && err == nil {
		f.Log.Info("Cleaning assets")
		cmd = exec.Command("bundle", "exec", "rake", "assets:clean")
		cmd.Dir = f.Stager.BuildDir()
		cmd.Stdout = text.NewIndentWriter(os.Stdout, []byte("       "))
		cmd.Stderr = text.NewIndentWriter(os.Stderr, []byte("       "))
		err = cmd.Run()
	}

	return err
}

func (f *Finalizer) InstallPlugins() error {
	if f.Gem12Factor {
		return nil
	}

	if f.RailsVersion == 4 {
		if !(f.GemStdoutLogging && f.GemStaticAssets) {
			f.Log.Protip("Include 'rails_12factor' gem to enable all platform features", "https://devcenter.heroku.com/articles/rails-integration-gems")
		}
		return nil
	}

	if f.RailsVersion == 2 || f.RailsVersion == 3 {
		if err := f.installPluginStdoutLogger(); err != nil {
			return err
		}
		if err := f.installPluginServeStaticAssets(); err != nil {
			return err
		}
	}
	return nil
}

func (f *Finalizer) installPluginStdoutLogger() error {
	if f.GemStdoutLogging {
		return nil
	}

	f.Log.BeginStep("Injecting plugin 'rails_log_stdout'")

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
	if f.GemStaticAssets {
		return nil
	}

	f.Log.BeginStep("Injecting plugin 'rails3_serve_static_assets'")

	code := "Rails.application.class.config.serve_static_assets = true\n"

	if err := os.MkdirAll(filepath.Join(f.Stager.BuildDir(), "vendor", "plugins", "rails3_serve_static_assets"), 0755); err != nil {
		return fmt.Errorf("Error creating rails3_serve_static_assets plugin directory: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(f.Stager.BuildDir(), "vendor", "plugins", "rails3_serve_static_assets", "init.rb"), []byte(code), 0644); err != nil {
		return fmt.Errorf("Error writing rails3_serve_static_assets plugin file: %v", err)
	}
	return nil
}

func (f *Finalizer) BestPracticeWarnings() {
	if os.Getenv("RAILS_ENV") != "production" {
		f.Log.Warning("You are deploying to a non-production environment: %s", os.Getenv("RAILS_ENV"))
	}
}
