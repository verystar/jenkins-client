package casc_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/verystar/jenkins-client/pkg/casc"
	"github.com/verystar/jenkins-client/pkg/mock/mhttp"
	"go.uber.org/mock/gomock"
)

var _ = Describe("", func() {
	var (
		ctrl         *gomock.Controller
		roundTripper *mhttp.MockRoundTripper
		cascManager  casc.Manager
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		roundTripper = mhttp.NewMockRoundTripper(ctrl)
		cascManager = casc.Manager{}
		cascManager.RoundTripper = roundTripper
		cascManager.URL = "http://localhost"
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("normal cases", func() {
		casc.PrepareForSASCReload(roundTripper, cascManager.URL, "", "")
		casc.PrepareForSASCApply(roundTripper, cascManager.URL, "", "")
		casc.PrepareForSASCExport(roundTripper, cascManager.URL, "", "")
		casc.PrepareForSASCSchema(roundTripper, cascManager.URL, "", "")
		casc.PrepareForCheckNewSource(roundTripper, cascManager.URL, "", "")
		casc.PrepareForReplaceSource(roundTripper, cascManager.URL, "", "")

		reloadErr := cascManager.Reload()
		applyErr := cascManager.Apply()
		config, exportErr := cascManager.Export()
		schema, schemaErr := cascManager.Schema()
		newSourceErr := cascManager.CheckNewSource("source")
		replaceSourceErr := cascManager.Replace("source")

		Expect(reloadErr).NotTo(HaveOccurred())
		Expect(applyErr).NotTo(HaveOccurred())
		Expect(exportErr).NotTo(HaveOccurred())
		Expect(schemaErr).NotTo(HaveOccurred())
		Expect(newSourceErr).NotTo(HaveOccurred())
		Expect(replaceSourceErr).NotTo(HaveOccurred())

		Expect(config).To(Equal("sample"))
		Expect(schema).To(Equal("sample"))
	})

	Context("with error code", func() {
		BeforeEach(func() {
			casc.PrepareForSASCExportWithCode(roundTripper, cascManager.URL, "", "", 500)
			casc.PrepareForSASCSchemaWithCode(roundTripper, cascManager.URL, "", "", 500)
		})

		It("get error", func() {
			_, exportErr := cascManager.Export()
			_, schemaErr := cascManager.Schema()

			Expect(exportErr).To(HaveOccurred())
			Expect(schemaErr).To(HaveOccurred())
		})
	})
})
