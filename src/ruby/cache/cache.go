package cache

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func New(stager Stager, yaml YAML) (*Cache, error) {
	c := &Cache{
		buildDir: stager.BuildDir(),
		cacheDir: stager.CacheDir(),
		depDir:   filepath.Join(stager.DepDir()),
		metadata: Metadata{},
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
		return os.Rename(filepath.Join(c.cacheDir, "vendor_bundler"), filepath.Join(c.depDir, "vendor_bundler"))
	}
	return os.RemoveAll(filepath.Join(c.cacheDir, "vendor_bundler"))
}

func (c *Cache) Save() error {
	cmd := exec.Command("cp", "-al", filepath.Join(c.depDir, "vendor_bundler"), filepath.Join(c.cacheDir, "vendor_bundler"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Could not copy vendor_bundler: %v", err)
	}

	if err := c.yaml.Write(c.metadata_yml(), c.metadata); err != nil {
		return err
	}

	return nil
}

func (c *Cache) metadata_yml() string {
	return filepath.Join(c.cacheDir, "metadata.yml")
}
