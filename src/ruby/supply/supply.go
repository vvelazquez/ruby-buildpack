package supply

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"ruby/cache"
	"strings"

	"github.com/cloudfoundry/libbuildpack"
	"github.com/kr/text"
)

type Command interface {
	Execute(string, io.Writer, io.Writer, string, ...string) error
	Output(string, string, ...string) (string, error)
}

type Manifest interface {
	AllDependencyVersions(string) []string
	InstallDependency(libbuildpack.Dependency, string) error
	InstallOnlyVersion(string, string) error
}
type Versions interface {
	Engine() (string, error)
	Version() (string, error)
	JrubyVersion() (string, error)
	RubyEngineVersion() (string, error)
	HasGemVersion(gem string, constraints ...string) (bool, error)
}

type Stager interface {
	BuildDir() string
	DepDir() string
	DepsIdx() string
	LinkDirectoryInDepDir(string, string) error
	WriteEnvFile(string, string) error
	WriteProfileD(string, string) error
	SetStagingEnvironment() error
}

type Cache interface {
	Metadata() *cache.Metadata
	Restore() error
	Save() error
}

type Supplier struct {
	Stager   Stager
	Manifest Manifest
	Log      *libbuildpack.Logger
	Versions Versions
	Cache    Cache
	Command  Command
}

func Run(s *Supplier) error {
	s.Log.BeginStep("Supplying Ruby")

	if err := s.Cache.Restore(); err != nil {
		s.Log.Error("Unable to restore cache: %s", err.Error())
		return err
	}

	if err := s.CreateDefaultEnv(); err != nil {
		s.Log.Error("Unable to setup default environment: %s", err.Error())
		return err
	}

	if err := s.InstallBundler(); err != nil {
		s.Log.Error("Unable to install bundler: %s", err.Error())
		return err
	}

	engine, err := s.Versions.Engine()
	if err != nil {
		s.Log.Error("Unable to determine ruby engine: %s", err.Error())
		return err
	}

	var rubyVersion string
	if engine == "ruby" {
		rubyVersion, err = s.Versions.Version()
		if err != nil {
			s.Log.Error("Unable to determine ruby version: %s", err.Error())
			return err
		}

	} else if engine == "jruby" {
		rubyVersion, err = s.Versions.JrubyVersion()
		if err != nil {
			s.Log.Error("Unable to determine jruby version: %s", err.Error())
			return err
		}
		if err = s.InstallJVM(); err != nil {
			s.Log.Error("Unable to install JVM: %s", err.Error())
			return err
		}
	} else {
		s.Log.Error("Sorry, we do not support engine: %s", engine)
		return fmt.Errorf("Sorry, we do not support engine: %s", engine)
	}

	if err := s.InstallRuby(engine, rubyVersion); err != nil {
		s.Log.Error("Unable to install ruby: %s", err.Error())
		return err
	}

	if !s.HasNode() { // TODO If needs node (gem execjs or gem webpacker)
		// TODO choose 4.x vs 6.x based upon rails version
		if err := s.InstallNode("6.x"); err != nil {
			s.Log.Error("Unable to install node: %s", err.Error())
			return err
		}

		if err := s.InstallYarn(); err != nil {
			s.Log.Error("Unable to install node: %s", err.Error())
			return err
		}

		if err := s.InstallYarnDependencies(); err != nil {
			s.Log.Error("Unable to install yarn dependencies: %s", err.Error())
			return err
		}
	}

	if err := s.InstallGems(); err != nil {
		s.Log.Error("Unable to install gems: %s", err.Error())
		return err
	}

	if err := s.WriteProfileD(engine); err != nil {
		s.Log.Error("Unable to write profile.d: %s", err.Error())
		return err
	}

	s.Cache.Metadata().Stack = os.Getenv("CF_STACK")
	if err := s.Cache.Save(); err != nil {
		s.Log.Error("Unable to save cache: %s", err.Error())
		return err
	}

	if err := s.Stager.SetStagingEnvironment(); err != nil {
		s.Log.Error("Unable to setup environment variables: %s", err.Error())
		return err
	}

	return nil
}

func (s *Supplier) InstallYarn() error {
	exists, err := libbuildpack.FileExists(filepath.Join(s.Stager.BuildDir(), "yarn.lock"))
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	tempDir, err := ioutil.TempDir("", "node")
	if err != nil {
		return err
	}
	if err := s.Manifest.InstallOnlyVersion("yarn", tempDir); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(tempDir, "dist"), filepath.Join(s.Stager.DepDir(), "yarn")); err != nil {
		return err
	}
	return s.Stager.LinkDirectoryInDepDir(filepath.Join(s.Stager.DepDir(), "yarn", "bin"), "bin")
}

func (s *Supplier) InstallYarnDependencies() error {
	exists, err := libbuildpack.FileExists(filepath.Join(s.Stager.BuildDir(), "yarn.lock"))
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	exists, err = libbuildpack.FileExists(filepath.Join(s.Stager.BuildDir(), "bin/yarn"))
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	s.Log.BeginStep("Installing dependencies using yarn")

	return s.Command.Execute(
		s.Stager.BuildDir(),
		text.NewIndentWriter(os.Stdout, []byte("       ")),
		text.NewIndentWriter(os.Stderr, []byte("       ")),
		"bin/yarn", "install",
	)
}

func (s *Supplier) InstallBundler() error {
	if err := s.Manifest.InstallOnlyVersion("bundler", filepath.Join(s.Stager.DepDir(), "bundler")); err != nil {
		return err
	}
	os.Setenv("GEM_HOME", filepath.Join(s.Stager.DepDir(), "gem_home"))
	if err := s.Stager.WriteEnvFile("GEM_HOME", filepath.Join(s.Stager.DepDir(), "gem_home")); err != nil {
		return err
	}
	gemPath := strings.Join([]string{filepath.Join(s.Stager.DepDir(), "gem_home"), filepath.Join(s.Stager.DepDir(), "bundler")}, ":")
	os.Setenv("GEM_PATH", gemPath)
	if err := s.Stager.WriteEnvFile("GEM_PATH", gemPath); err != nil {
		return err
	}

	if err := s.Stager.LinkDirectoryInDepDir(filepath.Join(s.Stager.DepDir(), "bundler", "bin"), "bin"); err != nil {
		return err
	}

	return nil
}

func (s *Supplier) InstallNode(version string) error {
	var dep libbuildpack.Dependency

	tempDir, err := ioutil.TempDir("", "node")
	if err != nil {
		return err
	}
	nodeInstallDir := filepath.Join(s.Stager.DepDir(), "node")

	if version == "" {
		return fmt.Errorf("must supply node version")
	}

	versions := s.Manifest.AllDependencyVersions("node")
	ver, err := libbuildpack.FindMatchingVersion(version, versions)
	if err != nil {
		return err
	}
	dep.Name = "node"
	dep.Version = ver

	if err := s.Manifest.InstallDependency(dep, tempDir); err != nil {
		return err
	}

	if err := os.Rename(filepath.Join(tempDir, fmt.Sprintf("node-v%s-linux-x64", dep.Version)), nodeInstallDir); err != nil {
		return err
	}

	return s.Stager.LinkDirectoryInDepDir(filepath.Join(nodeInstallDir, "bin"), "bin")
}

func (s *Supplier) HasNode() bool {
	_, err := s.Command.Output(s.Stager.BuildDir(), "node", "--version")
	return err == nil
}

func (s *Supplier) InstallJVM() error {
	if exists, err := libbuildpack.FileExists(filepath.Join(s.Stager.BuildDir(), ".jdk")); err != nil {
		return err
	} else if exists {
		s.Log.Info("Using pre-installed JDK")
		return nil
	}

	jvmInstallDir := filepath.Join(s.Stager.DepDir(), "jvm")
	if err := s.Manifest.InstallOnlyVersion("openjdk1.8-latest", jvmInstallDir); err != nil {
		return err
	}
	if err := s.Stager.LinkDirectoryInDepDir(filepath.Join(jvmInstallDir, "bin"), "bin"); err != nil {
		return err
	}

	scriptContents := `
if ! [[ "${JAVA_OPTS}" == *-Xmx* ]]; then
  export JAVA_MEM=${JAVA_MEM:--Xmx${JVM_MAX_HEAP:-384}m}
fi
export JAVA_OPTS=${JAVA_OPTS:--Xss512k -XX:+UseCompressedOops -Dfile.encoding=UTF-8}
export JRUBY_OPTS=${JRUBY_OPTS:--Xcompile.invokedynamic=false}
`

	return s.Stager.WriteProfileD("jruby.sh", scriptContents)
}

func (s *Supplier) InstallRuby(name, version string) error {
	installDir := filepath.Join(s.Stager.DepDir(), "ruby")

	if err := s.Manifest.InstallDependency(libbuildpack.Dependency{Name: name, Version: version}, installDir); err != nil {
		return err
	}

	rakePath := filepath.Join(s.Stager.DepDir(), "ruby", "bin", "rake")
	rakeContent, err := ioutil.ReadFile(rakePath)
	if err != nil {
		return err
	}
	groups := strings.SplitN(string(rakeContent), "\n", 2)
	groups[0] = fmt.Sprintf("#!/usr/bin/env %s", name)
	if err := ioutil.WriteFile(rakePath, []byte(strings.Join(groups, "\n")), 0755); err != nil {
		return err
	}

	if err := os.Symlink("ruby", filepath.Join(s.Stager.DepDir(), "ruby", "bin", "ruby.exe")); err != nil {
		return err
	}
	return s.Stager.LinkDirectoryInDepDir(filepath.Join(s.Stager.DepDir(), "ruby", "bin"), "bin")
}

type IndentedWriter struct {
	w   io.Writer
	pad string
}

func (w *IndentedWriter) Write(p []byte) (n int, err error) {
	lines := strings.Split(string(p), "\n")
	for i, line := range lines {
		lines[i] = w.pad + line
	}
	p = []byte(strings.Join(lines, "\n"))
	return w.Write(p)
}

func (s *Supplier) InstallGems() error {
	// TODO Warn .bundle/config ruby.rb:490
	// TODO Warn windows Gemfile.lock ruby:500 (and remove Gemfile.lock)

	without := os.Getenv("BUNDLE_WITHOUT")
	if without == "" {
		without = "development:test"
	}
	// FROM RUBY :: "#{bundle_bin} install --without #{bundle_without} --path vendor/bundle --binstubs #{bundler_binstubs_path}"
	// NOTE: Skip binstubs since we should install them into app during finalize
	// TODO install binstubs during finalize
	args := []string{"install", "--without", without, "--jobs=4", "--retry=4", "--path", filepath.Join(s.Stager.DepDir(), "vendor_bundle")}
	exists, err := libbuildpack.FileExists(filepath.Join(s.Stager.BuildDir(), "Gemfile.lock"))
	if err != nil {
		return err
	}
	if exists {
		args = append(args, "--deployment")
	}

	version := s.Manifest.AllDependencyVersions("bundler")
	s.Log.BeginStep("Installing dependencies using bundler %s", version[0])
	s.Log.Info("Running: bundle %s", strings.Join(args, " "))

	cmd := exec.Command("bundle", args...)
	cmd.Dir = s.Stager.BuildDir()
	cmd.Stdout = text.NewIndentWriter(os.Stdout, []byte("       "))
	cmd.Stderr = text.NewIndentWriter(os.Stderr, []byte("       "))
	env := os.Environ()
	env = append(env, "NOKOGIRI_USE_SYSTEM_LIBRARIES=true")
	cmd.Env = env

	return cmd.Run()
}

func (s *Supplier) CreateDefaultEnv() error {
	var environmentDefaults = map[string]string{
		"RAILS_ENV": "production",
		"RACK_ENV":  "production",
	}

	for envVar, envDefault := range environmentDefaults {
		if os.Getenv(envVar) == "" {
			os.Setenv(envVar, envDefault)
			if err := s.Stager.WriteEnvFile(envVar, envDefault); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Supplier) WriteProfileD(engine string) error {
	s.Log.BeginStep("Creating runtime environment")

	rubyEngineVersion, err := s.Versions.RubyEngineVersion()
	if err != nil {
		return err
	}

	depsIdx := s.Stager.DepsIdx()
	scriptContents := fmt.Sprintf(`
export LANG=${LANG:-en_US.UTF-8}
export RAILS_ENV=${RAILS_ENV:-production}
export RACK_ENV=${RACK_ENV:-production}
export RAILS_SERVE_STATIC_FILES=${RAILS_SERVE_STATIC_FILES:-enabled}
export RAILS_LOG_TO_STDOUT=${RAILS_LOG_TO_STDOUT:-enabled}

export GEM_HOME=${GEM_HOME:-$DEPS_DIR/%s/gem_home}
export GEM_PATH=${GEM_PATH:-GEM_PATH=$DEPS_DIR/%s/vendor_bundle/%s/%s:$DEPS_DIR/%s/gem_home:$DEPS_DIR/%s/bundler}

## TODO Is this the right plan?
bundle config PATH "$DEPS_DIR/%s/vendor_bundle"
		`, depsIdx, depsIdx, engine, rubyEngineVersion, depsIdx, depsIdx, depsIdx)

	hasRails41, err := s.Versions.HasGemVersion("rails", ">=4.1.0.beta1")
	if err != nil {
		return err
	}
	if hasRails41 {
		metadata := s.Cache.Metadata()
		if metadata.SecretKeyBase == "" {
			metadata.SecretKeyBase, err = s.Command.Output(s.Stager.BuildDir(), "bundle", "exec", "rake", "secret")
			if err != nil {
				return fmt.Errorf("Running 'rake secret'", err)
			}
		}
		scriptContents += fmt.Sprintf("\nexport SECRET_KEY_BASE=${SECRET_KEY_BASE:-%s}\n", metadata.SecretKeyBase)
	}

	return s.Stager.WriteProfileD("ruby.sh", scriptContents)
}
