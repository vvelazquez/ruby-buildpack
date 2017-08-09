package finalize_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"ruby/finalize"

	"github.com/cloudfoundry/libbuildpack"
	"github.com/cloudfoundry/libbuildpack/ansicleaner"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:generate mockgen -source=finalize.go --destination=mocks_finalize_test.go --package=finalize_test

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
		mockVersions = NewMockVersions(mockCtrl)

		args := []string{buildDir, "", depsDir, depsIdx}
		stager := libbuildpack.NewStager(args, logger, &libbuildpack.Manifest{})

		finalizer = &finalize.Finalizer{
			Stager:   stager,
			Versions: mockVersions,
			Log:      logger,
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()

		err = os.RemoveAll(buildDir)
		Expect(err).To(BeNil())

		err = os.RemoveAll(depsDir)
		Expect(err).To(BeNil())
	})

	PIt("SECRET_KEY_BASE in rails >= 4.1", func() {})
	PIt("LANG defaults to en_US.UTF-8", func() {})

	Describe("Install plugins", func() {
		JustBeforeEach(func() {
			Expect(finalizer.InstallPlugins()).To(Succeed())
		})

		Context("rails 3", func() {
			BeforeEach(func() {
				finalizer.RailsVersion = 3
			})

			Context("has rails_12factor gem", func() {
				BeforeEach(func() {
					finalizer.Gem12Factor = true
				})
				It("installs no plugins", func() {
					Expect(filepath.Join(buildDir, "vendor", "plugins")).ToNot(BeADirectory())
				})
			})

			Context("does not have rails_12factor gem", func() {
				BeforeEach(func() {
					finalizer.Gem12Factor = false
				})

				Context("the app has the gem rails_stdout_logging", func() {
					BeforeEach(func() {
						finalizer.GemStdoutLogging = true
					})

					It("does not install the plugin rails_log_stdout", func() {
						Expect(filepath.Join(buildDir, "vendor", "plugins", "rails_log_stdout")).ToNot(BeADirectory())
					})
				})

				Context("the app has the gem rails_serve_static_assets", func() {
					BeforeEach(func() {
						finalizer.GemStaticAssets = true
					})

					It("does not install the plugin rails3_serve_static_assets", func() {
						Expect(filepath.Join(buildDir, "vendor", "plugins", "rails3_serve_static_assets")).ToNot(BeADirectory())
					})
				})

				Context("the app has neither above gem", func() {
					It("installs plugin rails3_serve_static_assets", func() {
						Expect(filepath.Join(buildDir, "vendor", "plugins", "rails3_serve_static_assets", "init.rb")).To(BeARegularFile())
					})

					It("installs plugin rails_log_stdout", func() {
						Expect(filepath.Join(buildDir, "vendor", "plugins", "rails_log_stdout", "init.rb")).To(BeARegularFile())
					})
				})
			})
		})

		Context("rails 4", func() {
			var helpMessage string
			BeforeEach(func() {
				helpMessage = "Include 'rails_12factor' gem to enable all platform features"
				finalizer.RailsVersion = 4
			})

			It("installs no plugins", func() {
				Expect(filepath.Join(buildDir, "vendor", "plugins")).ToNot(BeADirectory())
			})

			Context("has rails_12factor gem", func() {
				BeforeEach(func() { finalizer.Gem12Factor = true })
				It("do not suggest rails_12factor to user", func() {
					Expect(buffer.String()).ToNot(ContainSubstring(helpMessage))
				})
			})

			Context("has rails_serve_static_assets and rails_stdout_logging gems", func() {
				BeforeEach(func() {
					finalizer.GemStdoutLogging = true
					finalizer.GemStaticAssets = true
				})
				It("do not suggest rails_12factor to user", func() {
					Expect(buffer.String()).ToNot(ContainSubstring(helpMessage))
				})
			})

			Context("has rails_serve_static_assets gem, but NOT rails_stdout_logging gem", func() {
				BeforeEach(func() {
					finalizer.GemStaticAssets = true
				})
				It("suggest rails_12factor to user", func() {
					Expect(buffer.String()).To(ContainSubstring(helpMessage))
				})
			})

			Context("has rails_stdout_logging gem, but NOT rails_serve_static_assets gem", func() {
				BeforeEach(func() {
					finalizer.GemStdoutLogging = true
				})
				It("suggest rails_12factor to user", func() {
					Expect(buffer.String()).To(ContainSubstring(helpMessage))
				})
			})

			Context("has none of the above gems", func() {
				It("suggest rails_12factor to user", func() {
					Expect(buffer.String()).To(ContainSubstring(helpMessage))
				})
			})
		})
		Context("rails 5", func() {
			BeforeEach(func() {
				finalizer.RailsVersion = 5
			})
			It("do not suggest anything", func() {
				Expect(buffer.String()).To(Equal(""))
			})
			It("installs no plugins", func() {
				Expect(filepath.Join(buildDir, "vendor", "plugins")).ToNot(BeADirectory())
			})
		})
	})

	Describe("best practice warnings", func() {
		Context("RAILS_ENV == production", func() {
			BeforeEach(func() { os.Setenv("RAILS_ENV", "production") })
			AfterEach(func() { os.Setenv("RAILS_ENV", "") })

			It("does not warn the user", func() {
				finalizer.BestPracticeWarnings()
				Expect(buffer.String()).To(Equal(""))
			})
		})

		Context("RAILS_ENV != production", func() {
			BeforeEach(func() { os.Setenv("RAILS_ENV", "otherenv") })
			AfterEach(func() { os.Setenv("RAILS_ENV", "") })

			It("warns the user", func() {
				finalizer.BestPracticeWarnings()
				Expect(buffer.String()).To(ContainSubstring("You are deploying to a non-production environment: otherenv"))
			})
		})
	})
})
