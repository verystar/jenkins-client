package job

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/verystar/jenkins-client/pkg/mock/mhttp"
	"go.uber.org/mock/gomock"
)

var _ = Describe("status test", func() {
	var (
		ctrl         *gomock.Controller
		roundTripper *mhttp.MockRoundTripper
		statusClient JenkinsStatusClient

		username string
		password string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		roundTripper = mhttp.NewMockRoundTripper(ctrl)
		statusClient = JenkinsStatusClient{}
		statusClient.RoundTripper = roundTripper
		statusClient.URL = "http://localhost"

		username = "admin"
		password = "token"
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("Get status", func() {
		It("should success", func() {
			statusClient.UserName = username
			statusClient.Token = password

			PrepareGetStatus(roundTripper, statusClient.URL, username, password)

			status, err := statusClient.Get()
			Expect(err).To(BeNil())
			Expect(status).NotTo(BeNil())
			Expect(status.NodeName).To(Equal("master"))
			Expect(status.Version).To(Equal("version"))
		})
	})
})
