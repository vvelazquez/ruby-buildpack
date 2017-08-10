package cache

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cloudfoundry/libbuildpack"
)

type Metadata struct {
	Stack         string
	SecretKeyBase string
}

type Cache struct {
	buildDir string
	cacheDir string
	depDir   string
	metadata Metadata
	log      *libbuildpack.Logger
	yaml     YAML
}

type Stager interface {
	BuildDir() string
	CacheDir() string
	DepDir() string
}

type YAML interface {
	Load(file string, obj interface{}) error
	Write(dest string, obj interface{}) error
}

func New(stager Stager, log *libbuildpack.Logger, yaml YAML) (*Cache, error) {
	c := &Cache{
		buildDir: stager.BuildDir(),
		cacheDir: stager.CacheDir(),
		depDir:   filepath.Join(stager.DepDir()),
		metadata: Metadata{},
		log:      log,
		yaml:     yaml,
	}

	if err := yaml.Load(c.metadata_yml(), &c.metadata); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	return c, nil
}

func (c *Cache) Metadata() *Metadata {
	return &c.metadata
}

func (c *Cache) Restore() error {
	if c.metadata.Stack == os.Getenv("CF_STACK") {
		c.log.BeginStep("Restoring vendor_bundle from cache")
		return os.Rename(filepath.Join(c.cacheDir, "vendor_bundle"), filepath.Join(c.depDir, "vendor_bundle"))
	}
	if c.metadata.Stack != "" {
		c.log.BeginStep("Skipping restoring vendor_bundle from cache, stack changed from %s to %s", c.metadata.Stack, os.Getenv("CF_STACK"))
	}
	return os.RemoveAll(filepath.Join(c.cacheDir, "vendor_bundle"))
}

func (c *Cache) Save() error {
	c.log.BeginStep("Saving vendor_bundle to cache")

	cmd := exec.Command("cp", "-al", filepath.Join(c.depDir, "vendor_bundle"), filepath.Join(c.cacheDir, "vendor_bundle"))
	if output, err := cmd.CombinedOutput(); err != nil {
		c.log.Error(string(output))
		return fmt.Errorf("Could not copy vendor_bundle: %v", err)
	}

	if err := c.yaml.Write(c.metadata_yml(), c.metadata); err != nil {
		return err
	}

	return nil
}

func (c *Cache) metadata_yml() string {
	return filepath.Join(c.cacheDir, "metadata.yml")
}
