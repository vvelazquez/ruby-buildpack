package supply_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"ruby/supply"

	"github.com/cloudfoundry/libbuildpack"
	"github.com/cloudfoundry/libbuildpack/ansicleaner"
	gomock "github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	// . "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

//go:generate mockgen -source=supply.go --destination=mocks_test.go --package=supply_test

var _ = Describe("Supply", func() {
	PIt("MOST TESTS", func() {})

	var (
		err          error
		buildDir     string
		depsDir      string
		depsIdx      string
		supplier     *supply.Supplier
		logger       *libbuildpack.Logger
		buffer       *bytes.Buffer
		mockCtrl     *gomock.Controller
		mockManifest *MockManifest
		mockVersions *MockVersions
		mockCommand  *MockCommand
	)

	BeforeEach(func() {
		buildDir, err = ioutil.TempDir("", "ruby-buildpack.build.")
		Expect(err).To(BeNil())

		depsDir, err = ioutil.TempDir("", "ruby-buildpack.deps.")
		Expect(err).To(BeNil())

		depsIdx = "9"
		Expect(os.MkdirAll(filepath.Join(depsDir, depsIdx), 0755)).To(Succeed())

		buffer = new(bytes.Buffer)

		logger = libbuildpack.NewLogger(ansicleaner.New(buffer))

		mockCtrl = gomock.NewController(GinkgoT())
		mockManifest = NewMockManifest(mockCtrl)
		mockVersions = NewMockVersions(mockCtrl)
		mockCommand = NewMockCommand(mockCtrl)

		args := []string{buildDir, "", depsDir, depsIdx}
		stager := libbuildpack.NewStager(args, logger, &libbuildpack.Manifest{})

		supplier = &supply.Supplier{
			Stager:   stager,
			Manifest: mockManifest,
			Log:      logger,
			Versions: mockVersions,
			Command:  mockCommand,
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()

		err = os.RemoveAll(buildDir)
		Expect(err).To(BeNil())

		err = os.RemoveAll(depsDir)
		Expect(err).To(BeNil())
	})

	Describe("CreateDefaultEnv", func() {
		Describe("SecretKeyBase", func() {
			Context("Rails >= 4.1", func() {
				BeforeEach(func() {
					mockVersions.EXPECT().RubyEngineVersion().Return("2.3.19", nil)
					mockVersions.EXPECT().HasGemVersion("rails", ">=4.1.0.beta1").Return(true, nil)
					mockCommand.EXPECT().Output(buildDir, "bundle", "exec", "rake", "secret").Return("abcdef", nil)
				})
				It("writes default SECRET_KEY_BASE to profile.d", func() {
					Expect(supplier.WriteProfileD()).To(Succeed())
					contents, err := ioutil.ReadFile(filepath.Join(depsDir, depsIdx, "profile.d", "ruby.sh"))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(contents)).To(ContainSubstring("export SECRET_KEY_BASE=${SECRET_KEY_BASE:-abcdef}"))
				})
			})
			Context("NOT Rails >= 4.1", func() {
				BeforeEach(func() {
					mockVersions.EXPECT().RubyEngineVersion().Return("2.3.19", nil)
					mockVersions.EXPECT().HasGemVersion("rails", ">=4.1.0.beta1").Return(false, nil)
				})
				It("does not set default SECRET_KEY_BASE in profile.d", func() {
					Expect(supplier.WriteProfileD()).To(Succeed())
					contents, err := ioutil.ReadFile(filepath.Join(depsDir, depsIdx, "profile.d", "ruby.sh"))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(contents)).ToNot(ContainSubstring("SECRET_KEY_BASE"))
				})
			})
		})

		Describe("Default Rails ENVS", func() {
			BeforeEach(func() {
				mockVersions.EXPECT().RubyEngineVersion().Return("2.3.19", nil)
				mockVersions.EXPECT().HasGemVersion("rails", ">=4.1.0.beta1").Return(false, nil)
			})

			It("writes default RAILS_ENV to profile.d", func() {
				Expect(supplier.WriteProfileD()).To(Succeed())
				contents, err := ioutil.ReadFile(filepath.Join(depsDir, depsIdx, "profile.d", "ruby.sh"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(contents)).To(ContainSubstring("export RAILS_ENV=${RAILS_ENV:-production}"))
			})

			It("writes default RAILS_SERVE_STATIC_FILES to profile.d", func() {
				Expect(supplier.WriteProfileD()).To(Succeed())
				contents, err := ioutil.ReadFile(filepath.Join(depsDir, depsIdx, "profile.d", "ruby.sh"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(contents)).To(ContainSubstring("export RAILS_SERVE_STATIC_FILES=${RAILS_SERVE_STATIC_FILES:-enabled}"))
			})

			It("writes default RAILS_LOG_TO_STDOUT to profile.d", func() {
				Expect(supplier.WriteProfileD()).To(Succeed())
				contents, err := ioutil.ReadFile(filepath.Join(depsDir, depsIdx, "profile.d", "ruby.sh"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(contents)).To(ContainSubstring("export RAILS_LOG_TO_STDOUT=${RAILS_LOG_TO_STDOUT:-enabled}"))
			})

			It("writes default GEM_PATH to profile.d", func() {
				Expect(supplier.WriteProfileD()).To(Succeed())
				contents, err := ioutil.ReadFile(filepath.Join(depsDir, depsIdx, "profile.d", "ruby.sh"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(contents)).To(ContainSubstring("export GEM_PATH=${GEM_PATH:-GEM_PATH=$DEPS_DIR/9/vendor_bundle/ruby/2.3.19:$DEPS_DIR/9/gem_home:$DEPS_DIR/9/bundler}"))
			})
		})

		Describe("InstallYarn", func() {
			Context("app has yarn.lock file", func() {
				BeforeEach(func() {
					Expect(ioutil.WriteFile(filepath.Join(buildDir, "yarn.lock"), []byte("contents"), 0644)).To(Succeed())
				})
				It("installs yarn", func() {
					mockManifest.EXPECT().InstallOnlyVersion("yarn", gomock.Any()).Do(func(_, tempDir string) error {
						Expect(os.MkdirAll(filepath.Join(tempDir, "dist", "bin"), 0755)).To(Succeed())
						Expect(ioutil.WriteFile(filepath.Join(tempDir, "dist", "bin", "yarn"), []byte("contents"), 0644)).To(Succeed())
						return nil
					})
					Expect(supplier.InstallYarn()).To(Succeed())

					Expect(filepath.Join(depsDir, depsIdx, "bin", "yarn")).To(BeAnExistingFile())
					data, err := ioutil.ReadFile(filepath.Join(depsDir, depsIdx, "bin", "yarn"))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(data)).To(Equal("contents"))
				})
			})
			Context("app does not have a yarn.lock file", func() {
				It("does NOT install yarn", func() {
					Expect(supplier.InstallYarn()).To(Succeed())
					Expect(filepath.Join(depsDir, depsIdx, "bin", "yarn")).ToNot(BeAnExistingFile())
				})
			})
		})

		Describe("InstallYarnDependencies", func() {
			Context("app has yarn.lock nd bin/yarn files", func() {
				BeforeEach(func() {
					Expect(ioutil.WriteFile(filepath.Join(buildDir, "yarn.lock"), []byte("contents"), 0644)).To(Succeed())
					Expect(os.MkdirAll(filepath.Join(buildDir, "bin"), 0755)).To(Succeed())
					Expect(ioutil.WriteFile(filepath.Join(buildDir, "bin", "yarn"), []byte("executable"), 0755)).To(Succeed())
				})
				It("runs bin/yarn install", func() {
					mockCommand.EXPECT().Execute(buildDir, gomock.Any(), gomock.Any(), "bin/yarn", "install").Return(nil)
					Expect(supplier.InstallYarnDependencies()).To(Succeed())
				})
			})
			Context("app does not have a yarn.lock file", func() {
				BeforeEach(func() {
					Expect(ioutil.WriteFile(filepath.Join(buildDir, "yarn.lock"), []byte("contents"), 0644)).To(Succeed())
				})
				It("does NOT run yarn install", func() {
					Expect(supplier.InstallYarnDependencies()).To(Succeed())
				})
			})

			Context("app does not have a bin/yarn file", func() {
				BeforeEach(func() {
					Expect(os.MkdirAll(filepath.Join(buildDir, "bin"), 0755)).To(Succeed())
					Expect(ioutil.WriteFile(filepath.Join(buildDir, "bin", "yarn"), []byte("executable"), 0755)).To(Succeed())
				})
				It("does NOT run yarn install", func() {
					Expect(supplier.InstallYarnDependencies()).To(Succeed())
				})
			})
		})

		Describe("HasNode", func() {
			Context("node is already installed", func() {
				BeforeEach(func() {
					mockCommand.EXPECT().Output(buildDir, "node", "--version").Return("v8.2.1", nil)
				})
				It("returns true", func() {
					Expect(supplier.HasNode()).To(BeTrue())
				})
			})
			Context("node is not already installed", func() {
				BeforeEach(func() {
					mockCommand.EXPECT().Output(buildDir, "node", "--version").Return("", fmt.Errorf("could not find node"))
				})
				It("returns false", func() {
					Expect(supplier.HasNode()).To(BeFalse())
				})
			})
		})
	})
})
