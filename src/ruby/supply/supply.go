package supply

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"ruby/versions"
	"strings"

	"github.com/cloudfoundry/libbuildpack"
	"github.com/kr/text"
)

// type Command interface {
// 	Execute(string, io.Writer, io.Writer, string, ...string) error
// }

type Manifest interface {
	AllDependencyVersions(string) []string
	InstallDependency(libbuildpack.Dependency, string) error
	InstallOnlyVersion(string, string) error
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

type Supplier struct {
	Stager   Stager
	Manifest Manifest
	Log      *libbuildpack.Logger
	Versions *versions.Versions
	// Command            Command
}

func Run(s *Supplier) error {
	if err := s.CreateDefaultEnv(); err != nil {
		s.Log.Error("Unable to setup default environment: %s", err.Error())
		return err
	}

	if err := s.InstallBundler(); err != nil {
		s.Log.Error("Unable to install bundler: %s", err.Error())
		return err
	}

	rubyVersion, err := s.Versions.Version()
	if err != nil {
		s.Log.Error("Unable to determine ruby version: %s", err.Error())
		return err
	}

	if err := s.InstallRuby(rubyVersion); err != nil {
		s.Log.Error("Unable to install ruby: %s", err.Error())
		return err
	}

	// TODO not all apps need this
	if err := s.InstallNode("6.x"); err != nil {
		s.Log.Error("Unable to install node: %s", err.Error())
		return err
	}

	// TODO not all apps need this
	if err := s.InstallYarn(); err != nil {
		s.Log.Error("Unable to install node: %s", err.Error())
		return err
	}

	if err := s.InstallGems(); err != nil {
		s.Log.Error("Unable to install gems: %s", err.Error())
		return err
	}

	if err := s.Stager.SetStagingEnvironment(); err != nil {
		s.Log.Error("Unable to setup environment variables: %s", err.Error())
		os.Exit(11)
	}

	return nil
}

func (s *Supplier) InstallYarn() error {
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

	if err := s.Stager.WriteProfileD("bundler.sh", fmt.Sprintf("export GEM_HOME=${GEM_HOME:-$DEPS_DIR/%s/gem_home}\nexport GEM_PATH=${GEM_PATH:-$DEPS_DIR/%s/gem_home:$DEPS_DIR/%s/bundler}\n", s.Stager.DepsIdx(), s.Stager.DepsIdx(), s.Stager.DepsIdx())); err != nil {
		return err
	}

	if err := s.Stager.LinkDirectoryInDepDir(filepath.Join(s.Stager.DepDir(), "bundler", "bin"), "bin"); err != nil {
		return err
	}

	// TODO Add above to profile.d dir (handle existing/missing)

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

func (s *Supplier) InstallRuby(version string) error {
	installDir := filepath.Join(s.Stager.DepDir(), "ruby")

	if err := s.Manifest.InstallDependency(libbuildpack.Dependency{Name: "ruby", Version: version}, installDir); err != nil {
		return err
	}

	rakePath := filepath.Join(s.Stager.DepDir(), "ruby", "bin", "rake")
	rakeContent, err := ioutil.ReadFile(rakePath)
	if err != nil {
		return err
	}
	groups := strings.SplitN(string(rakeContent), "\n", 2)
	groups[0] = "#!/usr/bin/env ruby"
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

	s.Log.BeginStep("Creating runtime environment")

	for envVar, envDefault := range environmentDefaults {
		if os.Getenv(envVar) == "" {
			os.Setenv(envVar, envDefault)
			if err := s.Stager.WriteEnvFile(envVar, envDefault); err != nil {
				return err
			}
		}
	}

	scriptContents := `
export RAILS_ENV=${RAILS_ENV:-production}
export RACK_ENV=${RACK_ENV:-production}
`

	return s.Stager.WriteProfileD("ruby.sh", scriptContents)
}
