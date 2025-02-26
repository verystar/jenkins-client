package queue

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/verystar/jenkins-client/pkg/core"
	"github.com/verystar/jenkins-client/pkg/mock/mhttp"
	"go.uber.org/mock/gomock"
)

var _ = Describe("queue test", func() {
	var (
		ctrl         *gomock.Controller
		roundTripper *mhttp.MockRoundTripper
		queueClient  Client
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		roundTripper = mhttp.NewMockRoundTripper(ctrl)
		queueClient = Client{}
		queueClient.RoundTripper = roundTripper
		queueClient.URL = "http://localhost"
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("get queue", func() {
		It("should success", func() {
			core.PrepareGetQueue(roundTripper, queueClient.URL, "", "")

			queue, err := queueClient.Get()
			Expect(err).To(BeNil())
			Expect(queue).NotTo(BeNil())
			Expect(len(queue.Items)).To(Equal(1))
			Expect(queue.Items[0].ID).To(Equal(62))
		})
	})

	Context("cancel", func() {
		It("should success", func() {
			core.PrepareCancelQueue(roundTripper, queueClient.URL, "", "")

			err := queueClient.Cancel(1)
			Expect(err).To(BeNil())
		})
	})
})
