package supply_test

import (
	"bytes"
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
					mockVersions.EXPECT().HasGemVersion("rails", ">=4.1.0.beta1").Return(true, nil)
					mockCommand.EXPECT().Output(buildDir, "rake", "secret").Return("abcdef", nil)
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
		})
	})
})
