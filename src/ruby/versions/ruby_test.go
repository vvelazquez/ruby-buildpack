package versions_test

import (
	"io/ioutil"
	"path/filepath"
	"ruby/versions"

	"github.com/cloudfoundry/libbuildpack"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:generate mockgen -source=ruby.go --destination=mocks_ruby_test.go --package=versions_test

var _ = Describe("Ruby", func() {
	var (
		mockCtrl *gomock.Controller
		manifest *MockManifest
		tmpDir   string
	)
	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		manifest = NewMockManifest(mockCtrl)

		var err error
		tmpDir, err = ioutil.TempDir("", "versions.ruby")
		Expect(err).ToNot(HaveOccurred())
	})
	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("Engine", func() {
		Context("Gemfile has a mri", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte(`ruby "~>2.2.0"`), 0644)).To(Succeed())
			})

			It("returns ruby", func() {
				v := versions.New(tmpDir, manifest)
				Expect(v.Engine()).To(Equal("ruby"))
			})
		})

		Context("Gemfile has jruby", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte(`ruby '2.2.3', :engine => 'jruby', :engine_version => '9.1.12.0'`), 0644)).To(Succeed())
			})

			It("returns jruby", func() {
				v := versions.New(tmpDir, manifest)
				Expect(v.Engine()).To(Equal("jruby"))
			})
		})

		Context("Gemfile has no constraint", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte(``), 0644)).To(Succeed())
			})

			It("returns ruby", func() {
				v := versions.New(tmpDir, manifest)
				Expect(v.Engine()).To(Equal("ruby"))
			})
		})
	})

	FDescribe("Version", func() {
		Context("Gemfile has a constraint", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte(`ruby "~>2.2.0"`), 0644)).To(Succeed())
			})

			It("returns highest matching version", func() {
				manifest.EXPECT().AllDependencyVersions("ruby").Return([]string{"1.2.3", "2.2.3", "2.2.4", "2.2.1", "3.1.2"})
				v := versions.New(tmpDir, manifest)
				Expect(v.Version()).To(Equal("2.2.4"))
			})

			It("errors if no matching versions", func() {
				manifest.EXPECT().AllDependencyVersions("ruby").Return([]string{"1.2.3", "3.1.2"})
				v := versions.New(tmpDir, manifest)
				_, err := v.Version()
				Expect(err).To(MatchError("Running ruby: No Matching ruby versions"))
			})
		})

		Context("Gemfile has no constraint", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte(``), 0644)).To(Succeed())
			})

			It("returns the default version from the manifest", func() {
				manifest.EXPECT().AllDependencyVersions("ruby").Return([]string{"1.2.3", "2.2.3", "2.2.4", "2.2.1", "3.1.2"})
				manifest.EXPECT().DefaultVersion("ruby").Return(libbuildpack.Dependency{Version: "3.2.1"}, nil)
				v := versions.New(tmpDir, manifest)
				version, err := v.Version()
				Expect(err).NotTo(HaveOccurred())
				Expect(version).To(Equal("3.2.1"))
			})
		})

		PIt("BUNDLE_GEMFILE env var is set", func() {})
	})

	Describe("JrubyVersion", func() {
		Context("Gemfile has a constraint", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte(`ruby '2.3.3', :engine => 'jruby', :engine_version => '9.1.12.0'`), 0644)).To(Succeed())
			})
			It("returns the requested version", func() {
				v := versions.New(tmpDir, manifest)
				Expect(v.JrubyVersion()).To(Equal("ruby-2.3.3-jruby-9.1.12.0"))
			})
		})

		PIt("BUNDLE_GEMFILE env var is set", func() {})
	})

	Describe("RubyEngineVersion", func() {
		It("returns the gem simplified ruby version", func() {
			v := versions.New(tmpDir, manifest)
			version, err := v.RubyEngineVersion()
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(MatchRegexp("^\\d+\\.\\d+.0$"))
		})
	})

	Describe("HasGem", func() {
		BeforeEach(func() {
			Expect(ioutil.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte(`gem 'roda'`), 0644)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(tmpDir, "Gemfile.lock"), []byte(`GEM
  specs:
    rack (2.0.3)
    roda (2.28.0)
      rack

PLATFORMS
  ruby

DEPENDENCIES
  roda

BUNDLED WITH
   1.15.3
			`), 0644)).To(Succeed())
		})

		It("returns true for roda", func() {
			v := versions.New(tmpDir, manifest)
			Expect(v.HasGem("roda")).To(BeTrue())
		})

		It("returns false for rails", func() {
			v := versions.New(tmpDir, manifest)
			Expect(v.HasGem("rails")).To(BeFalse())
		})
	})

	Describe("GemMajorVersion", func() {
		BeforeEach(func() {
			Expect(ioutil.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte(`gem 'roda'`), 0644)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(tmpDir, "Gemfile.lock"), []byte(`GEM
  specs:
    rack (2.0.3)
    roda (4.28.0.beta1)
      rack

PLATFORMS
  ruby

DEPENDENCIES
  roda
			`), 0644)).To(Succeed())
		})

		It("returns 2 for rack", func() {
			v := versions.New(tmpDir, manifest)
			Expect(v.GemMajorVersion("rack")).To(Equal(2))
		})

		It("returns 4 for roda", func() {
			v := versions.New(tmpDir, manifest)
			Expect(v.GemMajorVersion("roda")).To(Equal(4))
		})

		It("returns -1 for rails", func() {
			v := versions.New(tmpDir, manifest)
			Expect(v.GemMajorVersion("rails")).To(Equal(-1))
		})
	})

	Describe("HasGemVersion", func() {
		BeforeEach(func() {
			Expect(ioutil.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte(`gem 'roda'`), 0644)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(tmpDir, "Gemfile.lock"), []byte(`GEM
  specs:
    rack (2.0.3)
    roda (2.28.0)
      rack

PLATFORMS
  ruby

DEPENDENCIES
  roda

BUNDLED WITH
   1.15.3
			`), 0644)).To(Succeed())
		})

		It("returns true for >=2.28.0 for roda", func() {
			v := versions.New(tmpDir, manifest)
			match, err := v.HasGemVersion("roda", ">=2.28.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(match).To(BeTrue())
		})

		It("returns false for <2.28.0 for roda", func() {
			v := versions.New(tmpDir, manifest)
			match, err := v.HasGemVersion("roda", "<2.28.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(match).To(BeFalse())
		})

		It("returns true for >=2.2.0, <=3.0.0 for roda", func() {
			v := versions.New(tmpDir, manifest)
			match, err := v.HasGemVersion("roda", ">=2.2.0", "<=3.0.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(match).To(BeTrue())
		})

		It("returns false for >=2.2.0, <=2.3.0 for roda", func() {
			v := versions.New(tmpDir, manifest)
			match, err := v.HasGemVersion("roda", ">=2.2.0", "<=2.3.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(match).To(BeFalse())
		})

		It("returns false for rails", func() {
			v := versions.New(tmpDir, manifest)
			match, err := v.HasGemVersion("rails", "1.0.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(match).To(BeFalse())
		})
	})
})
