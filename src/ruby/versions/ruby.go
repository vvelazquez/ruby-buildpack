package versions

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry/libbuildpack"
)

type Manifest interface {
	AllDependencyVersions(string) []string
	DefaultVersion(string) (libbuildpack.Dependency, error)
}

type Versions struct {
	buildDir string
	manifest Manifest
}

func New(buildDir string, manifest Manifest) *Versions {
	return &Versions{
		buildDir: buildDir,
		manifest: manifest,
	}
}

func (v *Versions) Version() (string, error) {
	gemfile := filepath.Join(v.buildDir, "Gemfile")

	satisfied := fmt.Sprintf(`
		require 'json'
		require 'bundler'
		begin
		b = Bundler::Dsl.evaluate('%s', '%s.lock', {}).ruby_version
		if !b
			puts({error:nil, version:''}.to_json)
			exit 0
		end
		potentials = JSON.parse(STDIN.read)
		r = Gem::Requirement.create(b.versions)
		version = potentials.select { |v| r.satisfied_by? Gem::Version.new(v) }.sort.last
		if version
			puts({error:nil, version:version}.to_json)
		else
			puts({error:'No Matching ruby versions', version:nil}.to_json)
		end
		rescue => e
			puts({error:e.to_s, version:nil}.to_json)
		end
	`, filepath.Base(gemfile), filepath.Base(gemfile))

	versions := v.manifest.AllDependencyVersions("ruby")
	data, err := json.Marshal(versions)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("ruby", "-e", satisfied)
	cmd.Dir = filepath.Dir(gemfile)
	cmd.Stdin = strings.NewReader(string(data))
	cmd.Stderr = os.Stderr
	body, err := cmd.Output()
	if err != nil {
		return "", err
	}
	output := struct {
		Error   string `json:"error"`
		Version string `json:"version"`
	}{}
	if err := json.Unmarshal(body, &output); err != nil {
		return "", err
	}
	if output.Error != "" {
		return "", fmt.Errorf("Determining ruby version: %s", output.Error)
	}
	if output.Version == "" {
		// TODO warning about no version set by dev https://github.com/cloudfoundry/ruby-buildpack/blob/master/lib/language_pack/ruby.rb#L367-L372
		dep, err := v.manifest.DefaultVersion("ruby")
		return dep.Version, err
	}
	return output.Version, nil
}
