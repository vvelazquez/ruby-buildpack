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
		err       error
		buildDir  string
		depsDir   string
		depsIdx   string
		finalizer *finalize.Finalizer
		logger    *libbuildpack.Logger
		buffer    *bytes.Buffer
		mockCtrl  *gomock.Controller
		// mockManifest *MockManifest
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

		args := []string{buildDir, "", depsDir, depsIdx}
		stager := libbuildpack.NewStager(args, logger, &libbuildpack.Manifest{})

		finalizer = &finalize.Finalizer{
			Stager: stager,
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

	PIt("SECRET_KEY_BASE in rails >= 4.1", func() {})
	PIt("LANG defaults to en_US.UTF-8", func() {})
})
