package versions

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cloudfoundry/libbuildpack"
)

type Manifest interface {
	AllDependencyVersions(string) []string
	DefaultVersion(string) (libbuildpack.Dependency, error)
}

type Versions struct {
	buildDir    string
	manifest    Manifest
	cachedSpecs map[string]string
}

func New(buildDir string, manifest Manifest) *Versions {
	return &Versions{
		buildDir: buildDir,
		manifest: manifest,
	}
}

type output struct {
	Error  string      `json:"error"`
	Output interface{} `json:"output"`
}

func (v *Versions) Version() (string, error) {
	versions := v.manifest.AllDependencyVersions("ruby")
	gemfile := v.gemfile()
	code := fmt.Sprintf(`
		b = Bundler::Dsl.evaluate('%s', '%s.lock', {}).ruby_version
	  return '' if !b

		r = Gem::Requirement.create(b.versions)
		version = input.select { |v| r.satisfied_by? Gem::Version.new(v) }.sort.last
		raise 'No Matching ruby versions' unless version
		version
	`, filepath.Base(gemfile), filepath.Base(gemfile))

	data, err := v.run(filepath.Dir(gemfile), code, versions)
	if err != nil {
		return "", err
	}

	version := data.(string)
	if version == "" {
		// TODO warning about no version set by dev https://github.com/cloudfoundry/ruby-buildpack/blob/master/lib/language_pack/ruby.rb#L367-L372
		dep, err := v.manifest.DefaultVersion("ruby")
		return dep.Version, err
	}
	return version, nil
}

func (v *Versions) RubyEngineVersion() (string, error) {
	code := `require 'rbconfig';RbConfig::CONFIG['ruby_version']`

	data, err := v.run(v.buildDir, code, []string{})
	if err != nil {
		return "", err
	}
	return data.(string), nil
}

func (v *Versions) HasGemVersion(gem string, constraints ...string) (bool, error) {
	specs, err := v.specs()
	if err != nil {
		return false, err
	}
	if specs[gem] == "" {
		return false, nil
	}

	code := `
		gem_version = input.shift
		Gem::Requirement.create(input).satisfied_by? Gem::Version.new(gem_version)
	`

	data, err := v.run(v.buildDir, code, append([]string{specs[gem]}, constraints...))
	if err != nil {
		return false, err
	}

	return data.(bool), nil
}

func (v *Versions) HasGem(gem string) (bool, error) {
	specs, err := v.specs()
	if err != nil {
		return false, err
	}
	if specs[gem] != "" {
		return true, nil
	}
	return false, nil
}

func (v *Versions) GemMajorVersion(gem string) (int, error) {
	specs, err := v.specs()
	if err != nil {
		return -1, err
	}
	if specs[gem] == "" {
		return -1, nil
	}

	code := `Gem::Version.new(input.first).segments.first.to_s`
	data, err := v.run(v.buildDir, code, []string{specs[gem]})
	if err != nil {
		return -1, err
	}

	if i, err := strconv.Atoi(data.(string)); err == nil {
		return i, nil
	} else {
		return -1, err
	}
}

func (v *Versions) specs() (map[string]string, error) {
	if len(v.cachedSpecs) > 0 {
		return v.cachedSpecs, nil
	}
	code := `
		parsed = Bundler::LockfileParser.new(File.read(input["gemfilelock"]))
		Hash[*(parsed.specs.map{|spec| [spec.name, spec.version.to_s]}).flatten]
	`

	data, err := v.run(filepath.Dir(v.gemfile()), code, map[string]string{"gemfilelock": fmt.Sprintf("%s.lock", v.gemfile())})
	if err != nil {
		return nil, err
	}
	specs := make(map[string]string, 0)
	for k, v := range data.(map[string]interface{}) {
		specs[k] = v.(string)
	}
	v.cachedSpecs = specs
	return v.cachedSpecs, nil
}

func (v *Versions) gemfile() string {
	return filepath.Join(v.buildDir, "Gemfile")
}

func (v *Versions) run(dir, code string, in interface{}) (interface{}, error) {
	data, err := json.Marshal(in)
	if err != nil {
		return "", err
	}

	code = fmt.Sprintf(`
		begin
			def data(input)
				%s
			end
			input = JSON.parse(STDIN.read)
			out = data(input)
			puts({error:nil, data:out}.to_json)
		rescue => e
			puts({error:e.to_s, data:nil}.to_json)
		end
	`, code)

	cmd := exec.Command("ruby", "-rjson", "-rbundler", "-e", code)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(string(data))
	cmd.Stderr = os.Stderr
	body, err := cmd.Output()
	if err != nil {
		fmt.Println(body)
		return "", err
	}
	output := struct {
		Error string      `json:"error"`
		Data  interface{} `json:"data"`
	}{}
	if err := json.Unmarshal(body, &output); err != nil {
		return "", err
	}
	if output.Error != "" {
		return "", fmt.Errorf("Running ruby: %s", output.Error)
	}
	return output.Data, nil
}
