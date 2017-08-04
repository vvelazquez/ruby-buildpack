package finalize_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"ruby/finalize"

	"github.com/blang/semver"
	"github.com/cloudfoundry/libbuildpack"
	"github.com/cloudfoundry/libbuildpack/ansicleaner"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:generate mockgen -source=release.go --destination=mocks_release_test.go --package=finalize_test

var _ = Describe("Finalize", func() {
	var (
		err          error
		buildDir     string
		depsDir      string
		depsIdx      string
		finalizer    *finalize.Finalizer
		logger       *libbuildpack.Logger
		buffer       *bytes.Buffer
		mockCtrl     *gomock.Controller
		mockStager   *MockStager
		mockVersions *MockVersions
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
		// mockManifest = NewMockManifest(mockCtrl)
		mockStager = NewMockStager(mockCtrl)
		mockVersions = NewMockVersions(mockCtrl)

		// args := []string{buildDir, "", depsDir, depsIdx}
		// stager := libbuildpack.NewStager(args, logger, &libbuildpack.Manifest{})

		finalizer = &finalize.Finalizer{
			Stager:   mockStager,
			Versions: mockVersions,
			// Manifest: mockManifest,
			Log: logger,
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()

		err = os.RemoveAll(buildDir)
		Expect(err).To(BeNil())

		err = os.RemoveAll(depsDir)
		Expect(err).To(BeNil())
	})

	FDescribe("GenerateReleaseYaml", func() {
		Context("Rails 4+", func() {
			BeforeEach(func() {
				mockVersions.EXPECT().HasGem("thin").Return(true, nil)
				version4 := semver.MustParse("4.0.5")
				mockVersions.EXPECT().GemVersion("rails").Return(&version4, nil)
			})
			It("generates web, worker, rake and console process types", func() {
				data, err := finalizer.GenerateReleaseYaml()
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(Equal(map[string]map[string]string{
					"default_process_types": map[string]string{
						"rake":    "bundle exec rake",
						"console": "bin/rails console",
						"web":     "bin/rails server -b 0.0.0.0 -p $PORT -e $RAILS_ENV",
						"worker":  "bundle exec rake jobs:work",
					},
				}))
			})
		})
		Context("Rails 3.x", func() {
			BeforeEach(func() {
				version3 := semver.MustParse("3.5.0")
				mockVersions.EXPECT().GemVersion("rails").Return(&version3, nil)
			})
			Context("thin is not present", func() {
				BeforeEach(func() {
					mockVersions.EXPECT().HasGem("thin").Return(false, nil)
				})
				It("generates web, worker, rake and console process types", func() {
					data, err := finalizer.GenerateReleaseYaml()
					Expect(err).NotTo(HaveOccurred())
					Expect(data).To(Equal(map[string]map[string]string{
						"default_process_types": map[string]string{
							"rake":    "bundle exec rake",
							"console": "bundle exec rails console",
							"web":     "bundle exec rails server -p $PORT",
							"worker":  "bundle exec rake jobs:work",
						},
					}))
				})
			})
			Context("thin is present", func() {
				BeforeEach(func() {
					mockVersions.EXPECT().HasGem("thin").Return(true, nil)
				})
				It("generates web, worker, rake and console process types", func() {
					data, err := finalizer.GenerateReleaseYaml()
					Expect(err).NotTo(HaveOccurred())
					Expect(data).To(Equal(map[string]map[string]string{
						"default_process_types": map[string]string{
							"rake":    "bundle exec rake",
							"console": "bundle exec rails console",
							"web":     "bundle exec thin start -R config.ru -e $RAILS_ENV -p $PORT",
							"worker":  "bundle exec rake jobs:work",
						},
					}))
				})
			})
		})
		Context("Rails 2.x", func() {
			BeforeEach(func() {
				version2 := semver.MustParse("2.5.0")
				mockVersions.EXPECT().GemVersion("rails").Return(&version2, nil)
			})
			Context("thin is not present", func() {
				BeforeEach(func() {
					mockVersions.EXPECT().HasGem("thin").Return(false, nil)
				})
				It("generates web, worker, rake and console process types", func() {
					data, err := finalizer.GenerateReleaseYaml()
					Expect(err).NotTo(HaveOccurred())
					Expect(data).To(Equal(map[string]map[string]string{
						"default_process_types": map[string]string{
							"rake":    "bundle exec rake",
							"console": "bundle exec script/console",
							"web":     "bundle exec ruby script/server -p $PORT",
							"worker":  "bundle exec rake jobs:work",
						},
					}))
				})
			})
			Context("thin is present", func() {
				BeforeEach(func() {
					mockVersions.EXPECT().HasGem("thin").Return(true, nil)
				})
				It("generates web, worker, rake and console process types", func() {
					data, err := finalizer.GenerateReleaseYaml()
					Expect(err).NotTo(HaveOccurred())
					Expect(data).To(Equal(map[string]map[string]string{
						"default_process_types": map[string]string{
							"rake":    "bundle exec rake",
							"console": "bundle exec script/console",
							"web":     "bundle exec thin start -e $RAILS_ENV -p $PORT",
							"worker":  "bundle exec rake jobs:work",
						},
					}))
				})
			})
		})
		Context("Rack", func() {
			BeforeEach(func() {
				mockVersions.EXPECT().GemVersion("rails").Return(nil, nil)
				mockVersions.EXPECT().HasGem("rack").Return(true, nil)
			})
			Context("thin is not present", func() {
				BeforeEach(func() {
					mockVersions.EXPECT().HasGem("thin").Return(false, nil)
				})
				It("generates web, rake and console process types", func() {
					data, err := finalizer.GenerateReleaseYaml()
					Expect(err).NotTo(HaveOccurred())
					Expect(data).To(Equal(map[string]map[string]string{
						"default_process_types": map[string]string{
							"rake":    "bundle exec rake",
							"console": "bundle exec irb",
							"web":     "bundle exec rackup config.ru -p $PORT",
						},
					}))
				})
			})
			Context("thin is present", func() {
				BeforeEach(func() {
					mockVersions.EXPECT().HasGem("thin").Return(true, nil)
				})
				It("generates web, rake and console process types", func() {
					data, err := finalizer.GenerateReleaseYaml()
					Expect(err).NotTo(HaveOccurred())
					Expect(data).To(Equal(map[string]map[string]string{
						"default_process_types": map[string]string{
							"rake":    "bundle exec rake",
							"console": "bundle exec irb",
							"web":     "bundle exec thin start -R config.ru -e $RACK_ENV -p $PORT",
						},
					}))
				})
			})
		})
		Context("Ruby", func() {
			BeforeEach(func() {
				mockVersions.EXPECT().HasGem("thin").Return(false, nil)
				mockVersions.EXPECT().GemVersion("rails").Return(nil, nil)
				mockVersions.EXPECT().HasGem("rack").Return(false, nil)
			})
			It("generates rake and console process types", func() {
				data, err := finalizer.GenerateReleaseYaml()
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(Equal(map[string]map[string]string{
					"default_process_types": map[string]string{
						"rake":    "bundle exec rake",
						"console": "bundle exec irb",
					},
				}))
			})
		})
	})
})
