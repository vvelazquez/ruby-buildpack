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

		Context("has rails_12factor gem", func() {
			BeforeEach(func() { mockVersions.EXPECT().HasGem("rails_12factor").AnyTimes().Return(true, nil) })
			It("installs no plugins", func() {
				Expect(filepath.Join(buildDir, "vendor", "plugins")).ToNot(BeADirectory())
			})
		})

		Context("does not have rails_12factor gem", func() {
			BeforeEach(func() { mockVersions.EXPECT().HasGem("rails_12factor").AnyTimes().Return(false, nil) })

			Context("the app has the gem rails_stdout_logging", func() {
				BeforeEach(func() {
					mockVersions.EXPECT().HasGem("rails_serve_static_assets").AnyTimes().Return(false, nil)
					mockVersions.EXPECT().HasGem("rails_stdout_logging").AnyTimes().Return(true, nil)
				})

				It("does not install the plugin rails_log_stdout", func() {
					Expect(filepath.Join(buildDir, "vendor", "plugins", "rails_log_stdout")).ToNot(BeADirectory())
				})
			})

			Context("the app has the gem rails_serve_static_assets", func() {
				BeforeEach(func() {
					mockVersions.EXPECT().HasGem("rails_serve_static_assets").AnyTimes().Return(true, nil)
					mockVersions.EXPECT().HasGem("rails_stdout_logging").AnyTimes().Return(false, nil)
				})

				It("does not install the plugin rails3_serve_static_assets", func() {
					Expect(filepath.Join(buildDir, "vendor", "plugins", "rails3_serve_static_assets")).ToNot(BeADirectory())
				})
			})

			Context("the app has neither above gem", func() {
				BeforeEach(func() { mockVersions.EXPECT().HasGem(gomock.Any()).AnyTimes().Return(false, nil) })

				It("installs plugin rails3_serve_static_assets", func() {
					Expect(filepath.Join(buildDir, "vendor", "plugins", "rails3_serve_static_assets", "init.rb")).To(BeARegularFile())
				})

				It("installs plugin rails_log_stdout", func() {
					Expect(filepath.Join(buildDir, "vendor", "plugins", "rails_log_stdout", "init.rb")).To(BeARegularFile())
				})
			})
		})
	})
})
