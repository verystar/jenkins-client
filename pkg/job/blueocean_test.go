package job

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/verystar/jenkins-client/pkg/core"
	"github.com/verystar/jenkins-client/pkg/mock/mhttp"
	"go.uber.org/mock/gomock"
)

//go:embed testdata/PipelineRuns.json
var pipelineRunsDataSample string

var _ = Describe("SimplePipeline test via BlueOcean RESTful API", func() {
	var (
		ctrl         *gomock.Controller
		c            BlueOceanClient
		roundTripper *mhttp.MockRoundTripper
	)

	const organization = "jenkins"

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = BlueOceanClient{}
		roundTripper = mhttp.NewMockRoundTripper(ctrl)
		c.RoundTripper = roundTripper
		c.URL = "http://localhost"
		c.Organization = organization
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("Search", func() {
		given := func(name string, givenResponseJSON string) {
			api := fmt.Sprintf("/blue/rest/search/?q=pipeline:*%s*;type:pipeline;organization:%s;excludedFromFlattening=jenkins.branch.MultiBranchProject,com.cloudbees.hudson.plugins.folder.AbstractFolder&filter=no-folders&start=%d&limit=%d",
				name, organization, 0, 50)
			request, _ := http.NewRequest(http.MethodGet, api, nil)
			response := &http.Response{
				StatusCode: 200,
				Request:    request,
				Body:       ioutil.NopCloser(bytes.NewBufferString(givenResponseJSON)),
			}
			roundTripper.EXPECT().
				RoundTrip(core.NewRequestMatcher(request)).Return(response, nil)
		}

		It("Basic case with one result item", func() {
			name := "fake"
			given(name, `
[
  {
    "name": "fake",
    "displayName": "fake",
    "description": null,
    "type": "WorkflowJob",
    "shortURL": "job/fake/",
    "url": "job/fake/"
  }
]`)

			result, err := c.Search(name, 0, 50)
			Expect(err).To(BeNil())
			Expect(result).NotTo(BeNil())
			Expect(len(result)).To(Equal(1))
			Expect(result[0].Name).To(Equal("fake"))
		})

		It("Basic case without any result items", func() {
			name := "fake"
			given(name, `[]`)

			result, err := c.Search(name, 0, 50)
			Expect(err).To(BeNil())
			Expect(result).NotTo(BeNil())
			Expect(len(result)).To(Equal(0))
		})
	})

	Context("Build", func() {
		given := func(pipelineName string,
			branch string,
			statusCode int,
			requestCustomizer func(request *http.Request),
			responseCustomizer func(response *http.Response)) {
			api := c.getBuildAPI(BuildOption{
				Pipelines: []string{pipelineName},
				Branch:    branch,
			})

			request, _ := http.NewRequest(http.MethodPost, api, nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Add("CrumbRequestField", "Crumb")

			if requestCustomizer != nil {
				requestCustomizer(request)
			}

			response := &http.Response{
				StatusCode: statusCode,
				Proto:      "HTTP/1.1",
				Request:    request,
			}
			if responseCustomizer != nil {
				responseCustomizer(response)
			}

			roundTripper.EXPECT().RoundTrip(core.NewRequestMatcher(request).WithQuery().WithBody()).Return(response, nil)

			requestCrumb, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s%s", c.URL, "/crumbIssuer/api/json"), nil)
			responseCrumb := &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Request:    requestCrumb,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"crumbRequestField":"CrumbRequestField","crumb":"Crumb"}`)),
			}
			roundTripper.EXPECT().RoundTrip(core.NewRequestMatcher(requestCrumb).WithQuery().WithBody()).Return(responseCrumb, nil)
		}

		It("Trigger a simple SimplePipeline via Blue Ocean REST API", func() {
			const pipelineName = "fakePipeline"
			given(pipelineName, "", 200, nil, func(response *http.Response) {
				response.Body = io.NopCloser(strings.NewReader(`
{
  "expectedBuildNumber" : 1,
  "id" : "3",
  "enQueueTime": null
}`))
			})
			pipelineBuild, err := c.Build(BuildOption{
				Pipelines: []string{pipelineName},
			})
			Expect(err).To(BeNil())
			Expect(pipelineBuild).NotTo(BeNil())
			Expect(pipelineBuild.EnQueueTime.IsZero()).Should(BeTrue())
			Expect(pipelineBuild.ID).Should(Equal("3"))
		})

		It("Trigger a simple SimplePipeline with parameters", func() {
			const pipelineName = "fakePipeline"
			var parameters = []Parameter{{
				Name:  "this_is_a_name",
				Value: "this_is_a_value",
			}}
			paramBytes, err := json.Marshal(map[string][]Parameter{
				"parameters": parameters,
			})
			Expect(err).To(BeNil())

			given(pipelineName, "", 200, func(request *http.Request) {
				request.Body = io.NopCloser(strings.NewReader(string(paramBytes)))
			}, func(response *http.Response) {
				response.Body = io.NopCloser(strings.NewReader(`
{
 "expectedBuildNumber" : 1,
 "id" : "3",
 "enQueueTime": null
}`))
			})

			pipelineBuild, err := c.Build(BuildOption{
				Pipelines:  []string{pipelineName},
				Parameters: parameters,
			})

			Expect(err).To(BeNil())
			Expect(pipelineBuild).NotTo(BeNil())
			Expect(pipelineBuild.EnQueueTime.IsZero()).Should(BeTrue())
			Expect(pipelineBuild.ID).Should(Equal("3"))
		})

		It("Trigger a simple SimplePipeline with nil parameters", func() {
			const pipelineName = "fakePipeline"
			given(pipelineName, "", 200, func(request *http.Request) {
			}, func(response *http.Response) {
				response.Body = io.NopCloser(strings.NewReader(`
{
 "expectedBuildNumber" : 1,
 "id" : "3",
 "enQueueTime": null
}`))
			})

			pipelineBuild, err := c.Build(BuildOption{
				Pipelines:  []string{pipelineName},
				Parameters: nil,
			})

			Expect(err).To(BeNil())
			Expect(pipelineBuild).NotTo(BeNil())
			Expect(pipelineBuild.EnQueueTime.IsZero()).Should(BeTrue())
			Expect(pipelineBuild.ID).Should(Equal("3"))
		})

		It("Trigger a simple SimplePipeline with empty parameters", func() {
			const pipelineName = "fakePipeline"
			var parameters = []Parameter{}
			paramBytes, err := json.Marshal(map[string][]Parameter{
				"parameters": parameters,
			})
			Expect(err).To(BeNil())

			given(pipelineName, "", 200, func(request *http.Request) {
				request.Body = io.NopCloser(strings.NewReader(string(paramBytes)))
			}, func(response *http.Response) {
				response.Body = io.NopCloser(strings.NewReader(`
{
 "expectedBuildNumber" : 1,
 "id" : "3",
 "enQueueTime": null
}`))
			})

			pipelineBuild, err := c.Build(BuildOption{
				Pipelines:  []string{pipelineName},
				Parameters: parameters,
			})

			Expect(err).To(BeNil())
			Expect(pipelineBuild).NotTo(BeNil())
			Expect(pipelineBuild.EnQueueTime.IsZero()).Should(BeTrue())
			Expect(pipelineBuild.ID).Should(Equal("3"))
		})

		It("Trigger a simple SimplePipeline via Blue Ocean REST API with an error", func() {
			const pipelineName = "fakePipeline"
			given(pipelineName, "", 400, nil, func(response *http.Response) {
				response.Body = io.NopCloser(strings.NewReader(`
{
    "message": "parameters.name is required element",
    "code": 400,
    "errors": []
}`))
			})

			_, err := c.Build(BuildOption{
				Pipelines: []string{pipelineName},
			})
			Expect(err).NotTo(BeNil())
		})

		It("Trigger a multi branch SimplePipeline", func() {
			const (
				pipelineName = "fakePipeline"
				branch       = "feature-1"
			)
			given(pipelineName, "feature-1", 200, nil, func(response *http.Response) {
				response.Body = io.NopCloser(strings.NewReader(`
{
  "expectedBuildNumber" : 1,
  "id" : "3",
  "enQueueTime": null
}`))
			})

			pb, err := c.Build(BuildOption{
				Pipelines: []string{pipelineName},
				Branch:    branch,
			})
			Expect(err).To(BeNil())
			Expect(pb).NotTo(BeNil())
			Expect(pb.ID).Should(Equal("3"))
		})
	})

	Context("GetBuild", func() {
		given := func(pipelineName, runID string, branch string, statusCode int, givenResponseJSON string) {
			api := c.getGetBuildAPI(GetBuildOption{
				Pipelines: []string{pipelineName},
				Branch:    branch,
				RunID:     runID,
			})

			request, _ := http.NewRequest(http.MethodGet, api, nil)
			request.Header.Set("Content-Type", "application/json")
			response := &http.Response{
				StatusCode: statusCode,
				Proto:      "HTTP/1.1",
				Request:    request,
				Body:       io.NopCloser(bytes.NewBufferString(givenResponseJSON)),
			}
			roundTripper.EXPECT().RoundTrip(core.NewRequestMatcher(request)).Return(response, nil)
		}
		It("Get specific SimplePipeline run", func() {
			const (
				pipelineName = "fakePipeline"
				runID        = "1"
			)
			given(pipelineName, runID, "", 200, `
{
  "enQueueTime": "2021-08-25T07:29:13.483+0000",
  "endTime": null,
  "estimatedDurationInMillis": 35661,
  "id": "5",
  "result": "UNKNOWN",
  "startTime": "2021-08-25T07:29:13.499+0000",
  "state": "RUNNING"
}`)
			pipelineBuild, err := c.GetBuild(GetBuildOption{
				RunID:     runID,
				Pipelines: []string{pipelineName},
			})
			Expect(err).Should(BeNil())
			Expect(pipelineBuild).ShouldNot(BeNil())
		})
		It("Get specific SimplePipeline run with an error", func() {
			const (
				pipelineName = "fakePipeline"
				runID        = "1"
			)
			given(pipelineName, runID, "", 500, `
{
  "message" : "Failed to create Git pipeline: demo",
  "code" : 400,
  "errors" : [ {
    "message" : "demo already exists",
    "code" : "ALREADY_EXISTS",
    "field" : "name"
  } ]
}`)
			_, err := c.GetBuild(GetBuildOption{
				RunID:     runID,
				Pipelines: []string{pipelineName},
			})
			Expect(err).ShouldNot(BeNil())
		})
		It("Get multi branch SimplePipeline run", func() {
			const (
				pipelineName = "fakePipeline"
				runID        = "1"
				branch       = "main"
			)
			given(pipelineName, runID, branch, 200, `
{
  "enQueueTime": "2021-08-25T07:29:13.483+0000",
  "endTime": null,
  "estimatedDurationInMillis": 35661,
  "id": "5",
  "result": "UNKNOWN",
  "startTime": "2021-08-25T07:29:13.499+0000",
  "state": "RUNNING"
}`)
			pipelineBuild, err := c.GetBuild(GetBuildOption{
				RunID:     runID,
				Pipelines: []string{pipelineName},
				Branch:    branch,
			})
			Expect(err).Should(BeNil())
			Expect(pipelineBuild).ShouldNot(BeNil())
		})

	})

	Context("GetNodes", func() {
		given := func(option GetNodesOption, mockResponseStatus int, mockResponseData string) {
			api := c.getGetNodesAPI(option)
			mockRequest, err := http.NewRequest(http.MethodGet, api, nil)
			if err != nil {
				return
			}
			mockRequest.Header.Set("Content-Type", "application/json")
			mockResponse := &http.Response{
				StatusCode: mockResponseStatus,
				Proto:      "HTTP/1.1",
				Request:    mockRequest,
				Body:       io.NopCloser(bytes.NewBufferString(mockResponseData)),
			}
			roundTripper.EXPECT().RoundTrip(core.NewRequestMatcher(mockRequest)).Return(mockResponse, nil)
		}
		It("Get SimplePipeline nodes detail", func() {
			given(GetNodesOption{
				Pipelines: []string{"pipelineA"},
				RunID:     "123",
			}, 200, `
[
  {
    "displayName": "build",
    "durationInMillis": 219,
    "edges": [
      {
        "id": "9"
      }
    ],
    "id": "3",
    "result": "SUCCESS",
    "startTime": "2021-09-05T15:15:08.719-0700",
    "state": "FINISHED"
  }
]`)

			nodes, err := c.GetNodes(GetNodesOption{
				Pipelines: []string{"pipelineA"},
				RunID:     "123",
			})
			Expect(err).Should(BeNil())
			Expect(len(nodes)).Should(Equal(1))
			Expect(nodes[0].ID).Should(Equal("3"))
			Expect(nodes[0].Result).Should(Equal("SUCCESS"))
			Expect(nodes[0].StartTime.In(time.UTC)).Should(Equal(time.Date(2021, 9, 5, 22, 15, 8, 719000000, time.UTC)))
		})
		It("Get SimplePipeline nodes detail with error", func() {
			given(GetNodesOption{
				Pipelines: []string{"pipelineA"},
				RunID:     "456",
			}, 400, `
{
  "message": "Failed to get pipeline run nodes detail: pipelineA/456",
  "code": 400,
  "errors": [
    {
      "message": "pipeline run ID was not exist",
      "code": "NOT_EXISTS",
      "field": "runID"
    }
  ]
}`)

			_, err := c.GetNodes(GetNodesOption{
				Pipelines: []string{"pipelineA"},
				RunID:     "456",
			})
			Expect(err).ShouldNot(BeNil())
		})
	})

	Context("GetPipelines", func() {
		It("Without folders, and without data", func() {
			request, _ := http.NewRequest(http.MethodGet, "/blue/rest/organizations/jenkins/pipelines", nil)
			response := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("[]")),
			}
			roundTripper.EXPECT().
				RoundTrip(core.NewRequestMatcher(request)).
				Return(response, nil)

			pipelines, err := c.GetPipelines()
			Expect(err).To(BeNil())
			Expect(len(pipelines)).To(Equal(0))
		})

		It("Without folders, with one pipeline", func() {
			request, _ := http.NewRequest(http.MethodGet, "/blue/rest/organizations/jenkins/pipelines", nil)
			response := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`[{"_class":"io.jenkins.blueocean.rest.impl.pipeline.PipelineImpl","_links":{"self":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/sd/"},"scm":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/sd/scm/"},"actions":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/sd/actions/"},"runs":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/sd/runs/"},"trends":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/sd/trends/"},"queue":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/sd/queue/"}},"actions":[],"disabled":false,"displayName":"sd","estimatedDurationInMillis":-1,"fullDisplayName":"sd","fullName":"sd","latestRun":null,"name":"sd","organization":"jenkins","parameters":[],"permissions":{"create":true,"configure":true,"read":true,"start":true,"stop":true},"weatherScore":100}]`)),
			}
			roundTripper.EXPECT().
				RoundTrip(core.NewRequestMatcher(request)).
				Return(response, nil)

			pipelines, err := c.GetPipelines()
			Expect(err).To(BeNil())
			Expect(len(pipelines)).To(Equal(1))
			Expect(pipelines[0].Name).To(Equal("sd"))
		})

		It("Without one folder, with one pipeline", func() {
			name := "test"
			folder := "folder"
			request, _ := http.NewRequest(http.MethodGet, "/blue/rest/organizations/jenkins/pipelines/folder/pipelines/", nil)
			response := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`[{"_class":"io.jenkins.blueocean.rest.impl.pipeline.PipelineImpl","_links":{"self":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/folder/pipelines/test/"},"scm":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/folder/pipelines/test/scm/"},"actions":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/folder/pipelines/test/actions/"},"runs":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/folder/pipelines/test/runs/"},"trends":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/folder/pipelines/test/trends/"},"queue":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/folder/pipelines/test/queue/"}},"actions":[],"disabled":false,"displayName":"test","estimatedDurationInMillis":-1,"fullDisplayName":"folder/test","fullName":"folder/test","latestRun":null,"name":"test","organization":"jenkins","parameters":[],"permissions":{"create":true,"configure":true,"read":true,"start":true,"stop":true},"weatherScore":100}]`)),
			}
			roundTripper.EXPECT().
				RoundTrip(core.NewRequestMatcher(request)).
				Return(response, nil)

			pipelines, err := c.GetPipelines(folder)
			Expect(err).To(BeNil())
			Expect(len(pipelines)).To(Equal(1))
			Expect(pipelines[0].Name).To(Equal(name))
		})

		It("Without two folders, with one pipeline", func() {
			name := "test"
			folder1 := "folder1"
			folder2 := "folder2"
			request, _ := http.NewRequest(http.MethodGet, "/blue/rest/organizations/jenkins/pipelines/folder1/pipelines/folder2/pipelines/", nil)
			response := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`[{"_class":"io.jenkins.blueocean.rest.impl.pipeline.PipelineImpl","_links":{"self":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/folder/pipelines/test/"},"scm":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/folder/pipelines/test/scm/"},"actions":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/folder/pipelines/test/actions/"},"runs":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/folder/pipelines/test/runs/"},"trends":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/folder/pipelines/test/trends/"},"queue":{"_class":"io.jenkins.blueocean.rest.hal.Link","href":"/blue/rest/organizations/jenkins/pipelines/folder/pipelines/test/queue/"}},"actions":[],"disabled":false,"displayName":"test","estimatedDurationInMillis":-1,"fullDisplayName":"folder/test","fullName":"folder/test","latestRun":null,"name":"test","organization":"jenkins","parameters":[],"permissions":{"create":true,"configure":true,"read":true,"start":true,"stop":true},"weatherScore":100}]`)),
			}
			roundTripper.EXPECT().
				RoundTrip(core.NewRequestMatcher(request)).
				Return(response, nil)

			pipelines, err := c.GetPipelines(folder1, folder2)
			Expect(err).To(BeNil())
			Expect(len(pipelines)).To(Equal(1))
			Expect(pipelines[0].Name).To(Equal(name))
		})
	})

	Context("GetPipeline", func() {
		It("Without folder", func() {
			pipelineName := "pipelineA"
			request, _ := http.NewRequest(http.MethodGet, c.getGetPipelineAPI(pipelineName), nil)
			response := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"name":"pipelineA"}`)),
			}
			roundTripper.EXPECT().
				RoundTrip(core.NewRequestMatcher(request)).
				Return(response, nil)

			pipeline, err := c.GetPipeline(pipelineName)
			Expect(err).To(Succeed())
			Expect(pipeline.Name).To(Equal(pipelineName))
		})

		It("With one folder", func() {
			pipelineName := "pipelineA"
			folder1 := "folder1"
			request, _ := http.NewRequest(http.MethodGet, c.getGetPipelineAPI(pipelineName, folder1), nil)
			response := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"name":"pipelineA"}`)),
			}
			roundTripper.EXPECT().
				RoundTrip(core.NewRequestMatcher(request)).
				Return(response, nil)

			pipeline, err := c.GetPipeline(pipelineName, folder1)
			Expect(err).To(Succeed())
			Expect(pipeline.Name).To(Equal(pipelineName))
		})

		It("With two folders", func() {
			pipelineName := "pipelineA"
			folder1 := "folder1"
			folder2 := "folder2"
			request, _ := http.NewRequest(http.MethodGet, c.getGetPipelineAPI(pipelineName, folder1, folder2), nil)
			response := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"name":"pipelineA"}`)),
			}
			roundTripper.EXPECT().
				RoundTrip(core.NewRequestMatcher(request)).
				Return(response, nil)

			pipeline, err := c.GetPipeline(pipelineName, folder1, folder2)
			Expect(err).To(Succeed())
			Expect(pipeline.Name).To(Equal(pipelineName))
		})
	})

	Context("GetPipelineRun", func() {
		It("Without two folders, with one pipeline", func() {
			name := "test"
			folder1 := "folder1"
			folder2 := "folder2"
			request, _ := http.NewRequest(http.MethodGet, "/blue/rest/organizations/jenkins/pipelines/folder1/pipelines/folder2/pipelines/test/runs/", nil)
			response := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(pipelineRunsDataSample)),
			}
			roundTripper.EXPECT().
				RoundTrip(core.NewRequestMatcher(request)).
				Return(response, nil)

			pipelines, err := c.GetPipelineRuns(name, folder1, folder2)
			Expect(err).To(BeNil())
			Expect(len(pipelines)).To(Equal(1))
			Expect(pipelines[0].Result).To(Equal("SUCCESS"))
		})
	})

	Context("Replay", func() {
		given := func(option *ReplayOption, mockResponseStatus int, mockResponseData string) {
			api := c.getReplayAPI(option)
			mockRequest, _ := http.NewRequest(http.MethodPost, api, nil)
			mockRequest.Header.Set("Content-Type", "application/json")
			mockRequest.Header.Add("CrumbRequestField", "Crumb")
			mockResponse := &http.Response{
				StatusCode: mockResponseStatus,
				Proto:      "HTTP/1.1",
				Request:    mockRequest,
				Body:       io.NopCloser(bytes.NewBufferString(mockResponseData)),
			}
			roundTripper.EXPECT().RoundTrip(core.NewRequestMatcher(mockRequest).WithQuery().WithBody()).Return(mockResponse, nil)

			requestCrumb, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s%s", c.URL, "/crumbIssuer/api/json"), nil)
			responseCrumb := &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Request:    requestCrumb,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"crumbRequestField":"CrumbRequestField","crumb":"Crumb"}`)),
			}
			roundTripper.EXPECT().RoundTrip(core.NewRequestMatcher(requestCrumb).WithQuery().WithBody()).Return(responseCrumb, nil)
		}
		It("Replay PipelineRun", func() {
			given(&ReplayOption{
				Folders: []string{"pipelineA"},
				Branch:  "bug/ux-334",
				RunID:   "123",
			}, 200, `
{
  "_class" : "io.jenkins.blueocean.service.embedded.rest.QueueItemImpl",
  "id" : "64",
  "expectedBuildNumber" : 10,
  "pipeline" : "bug%2FUX-334",
  "_links" : {
     "self" : {
        "_class" : "io.jenkins.blueocean.rest.hal.Link",
        "href" : "/blue/rest/organizations/jenkins/pipelines/bo2/queue/64/"
     }
  },
  "queuedTime" : "2016-06-29T14:11:52.191-0700"
}`)

			pipelineRun, err := c.Replay(ReplayOption{
				Folders: []string{"pipelineA"},
				Branch:  "bug/ux-334",
				RunID:   "123",
			})
			Expect(err).Should(BeNil())
			Expect(pipelineRun).ShouldNot(BeNil())
			Expect(pipelineRun.ID).Should(Equal("64"))
		})

		It("Failed to relay PipelineRun", func() {
			given(&ReplayOption{
				Folders: []string{"pipelineA"},
				RunID:   "123",
			}, 400, `
{
    "message": "parameters.name is required element",
    "code": 400,
    "errors": []
}`)
			_, err := c.Replay(ReplayOption{
				Folders: []string{"pipelineA"},
				RunID:   "123",
			})
			Expect(err).ShouldNot(BeNil())
		})
	})

	Context("GetSteps", func() {
		given := func(api string, statusCode int, body string) {
			request, _ := http.NewRequest(http.MethodGet, api, nil)
			response := &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			}
			roundTripper.EXPECT().RoundTrip(core.NewRequestMatcher(request)).Return(response, nil)
		}
		It("Without folders", func() {
			option := GetStepsOption{
				RunID:        "123",
				PipelineName: "pipelineA",
			}
			given("/blue/rest/organizations/jenkins/pipelines/pipelineA/runs/123/steps/", 200, "[]")
			steps, err := c.GetSteps(option)
			Expect(err).To(Succeed())
			Expect(steps).NotTo(BeNil())
			Expect(len(steps)).To(Equal(0))
		})
		It("With one folder", func() {
			option := GetStepsOption{
				RunID:        "123",
				PipelineName: "pipelineA",
				Folders:      []string{"folder1"},
			}
			given("/blue/rest/organizations/jenkins/pipelines/folder1/pipelines/pipelineA/runs/123/steps/", 200, "[]")
			steps, err := c.GetSteps(option)
			Expect(err).To(Succeed())
			Expect(steps).NotTo(BeNil())
		})
		It("With two folders", func() {
			option := GetStepsOption{
				RunID:        "123",
				PipelineName: "pipelineA",
				Folders:      []string{"folder1", "folder2"},
			}
			given("/blue/rest/organizations/jenkins/pipelines/folder1/pipelines/folder2/pipelines/pipelineA/runs/123/steps/", 200, "[]")
			steps, err := c.GetSteps(option)
			Expect(err).To(Succeed())
			Expect(steps).NotTo(BeNil())
		})
		It("Without folder but with branch", func() {
			option := GetStepsOption{
				RunID:        "123",
				PipelineName: "pipelineA",
				Branch:       "main",
			}
			given("/blue/rest/organizations/jenkins/pipelines/pipelineA/branches/main/runs/123/steps/", 200, "[]")
			steps, err := c.GetSteps(option)
			Expect(err).To(Succeed())
			Expect(steps).NotTo(BeNil())
			Expect(len(steps)).To(Equal(0))
		})
		It("Without folder but with branch with special character", func() {
			option := GetStepsOption{
				RunID:        "123",
				PipelineName: "pipelineA",
				Branch:       "release%2Fv3.2",
			}
			given("/blue/rest/organizations/jenkins/pipelines/pipelineA/branches/release%252Fv3.2/runs/123/steps/", 200, "[]")
			steps, err := c.GetSteps(option)
			Expect(err).To(Succeed())
			Expect(steps).NotTo(BeNil())
			Expect(len(steps)).To(Equal(0))
		})
		It("With one folder and branch", func() {
			option := GetStepsOption{
				RunID:        "123",
				PipelineName: "pipelineA",
				Branch:       "main",
				Folders:      []string{"folder1"},
			}
			given("/blue/rest/organizations/jenkins/pipelines/folder1/pipelines/pipelineA/branches/main/runs/123/steps/", 200, "[]")
			steps, err := c.GetSteps(option)
			Expect(err).To(Succeed())
			Expect(steps).NotTo(BeNil())
			Expect(len(steps)).To(Equal(0))
		})
		It("Resposne one step", func() {
			option := GetStepsOption{
				RunID:        "123",
				PipelineName: "pipelineA",
			}
			given("/blue/rest/organizations/jenkins/pipelines/pipelineA/runs/123/steps/", 200, `
[{
  "displayName" : "Shell Script",
  "durationInMillis" : 70,
  "id" : "5",
  "result" : "SUCCESS",
  "startTime" : "2021-10-02T10:37:30.443+0800"
}]`)
			steps, err := c.GetSteps(option)
			Expect(err).To(Succeed())
			Expect(steps).NotTo(BeNil())
			Expect(len(steps)).To(Equal(1))
			Expect(steps[0].ID).To(Equal("5"))
			Expect(steps[0].StartTime.Local()).To(Equal(Time{Time: time.Date(2021, 10, 2, 2, 37, 30, 443000000, time.UTC)}.Local()))
			Expect(steps[0].Result).To(Equal("SUCCESS"))
			Expect(steps[0].DurationInMillis).To(Equal(int64(70)))
			Expect(steps[0].DisplayName).To(Equal("Shell Script"))
		})
		It("With node ID", func() {
			option := GetStepsOption{
				RunID:        "123",
				PipelineName: "pipelineA",
				NodeID:       "456",
			}
			given("/blue/rest/organizations/jenkins/pipelines/pipelineA/runs/123/nodes/456/steps/", 200, "[]")
			steps, err := c.GetSteps(option)
			Expect(err).To(Succeed())
			Expect(steps).NotTo(BeNil())
			Expect(len(steps)).To(Equal(0))
		})
	})

	Context("GetBranches", func() {
		given := func(api string, expectedStatus int, responseBody string) {
			request, _ := http.NewRequest(http.MethodGet, api, nil)
			response := &http.Response{
				StatusCode: expectedStatus,
				Body:       io.NopCloser(strings.NewReader(responseBody)),
			}
			roundTripper.EXPECT().RoundTrip(core.NewRequestMatcher(request)).Return(response, nil)
		}
		It("Without folder", func() {
			given("/blue/rest/organizations/jenkins/pipelines/pipelineA/branches/", http.StatusOK, `[]`)
			branches, err := c.GetBranches(GetBranchesOption{
				PipelineName: "pipelineA",
			})
			Expect(err).To(Succeed())
			Expect(branches).NotTo(BeNil())
			Expect(len(branches)).To(Equal(0))
		})
		It("With one folder", func() {
			given("/blue/rest/organizations/jenkins/pipelines/folder1/pipelines/pipelineA/branches/", http.StatusOK, `[]`)
			branches, err := c.GetBranches(GetBranchesOption{
				Folders:      []string{"folder1"},
				PipelineName: "pipelineA",
			})
			Expect(err).To(Succeed())
			Expect(branches).NotTo(BeNil())
			Expect(len(branches)).To(Equal(0))
		})
		It("With query", func() {
			given("/blue/rest/organizations/jenkins/pipelines/pipelineA/branches/?filter=origin&start=123&limit=456", http.StatusOK, `[]`)
			branches, err := c.GetBranches(GetBranchesOption{
				PipelineName: "pipelineA",
				Filter:       OriginFilter,
				Start:        123,
				Limit:        456,
			})
			Expect(err).To(Succeed())
			Expect(branches).NotTo(BeNil())
			Expect(len(branches)).To(Equal(0))
		})
		It("Response a branch", func() {
			given("/blue/rest/organizations/jenkins/pipelines/pipelineA/branches/", http.StatusOK, `
[{
   "disabled":false,
   "displayName":"v0.0.1",
   "estimatedDurationInMillis":-1,
   "fullDisplayName":"my-devops-projectsg945/github-pipeline/v0.0.1",
   "fullName":"my-devops-projectsg945/github-pipeline/v0.0.1",
   "latestRun":null,
   "name":"v0.0.1",
   "organization":"jenkins",
   "weatherScore":100,
   "branch":{
      "isPrimary":false,
      "issues":[],
      "url":"https://github.com/JohnNiang/devops-java-thin-sample/tree/v0.0.1"
   }
}]
			`)
			branches, err := c.GetBranches(GetBranchesOption{
				PipelineName: "pipelineA",
			})
			Expect(err).To(Succeed())
			Expect(branches).NotTo(BeNil())
			Expect(len(branches)).To(Equal(1))
			branch := branches[0]
			Expect(branch.Disabled).Should(BeFalse())
			Expect(branch.Name).Should(Equal("v0.0.1"))
			Expect(branch.WeatherScore).Should(Equal(100))
			Expect(branch.LatestRun).To(BeNil())
			Expect(branch.Branch.URL).Should(Equal("https://github.com/JohnNiang/devops-java-thin-sample/tree/v0.0.1"))
		})
		It("Error Respoonse", func() {
			given("/blue/rest/organizations/jenkins/pipelines/pipelineA/branches/", http.StatusBadRequest, `Invalid Pipeline name`)
			_, err := c.GetBranches(GetBranchesOption{
				PipelineName: "pipelineA",
			})
			Expect(err).NotTo(Succeed())
		})
	})
})

func Test_getHeaders(t *testing.T) {
	tests := []struct {
		name string
		want map[string]string
	}{{
		name: "Get headers",
		want: map[string]string{
			"Content-Type": "application/json",
		},
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getHeaders(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getHeaders() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlueOceanClient_getBuildAPI(t *testing.T) {
	type fields struct {
		Organization string
	}
	type args struct {
		option BuildOption
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{{
		name: "Get `Build` API",
		fields: fields{
			Organization: "jenkins",
		},
		args: args{
			BuildOption{
				Pipelines: []string{"pipelineA"},
			},
		},
		want: "/blue/rest/organizations/jenkins/pipelines/pipelineA/runs/",
	}, {
		name: "Get `Build` API with branch",
		fields: fields{
			Organization: "jenkins",
		},
		args: args{
			BuildOption{
				Pipelines: []string{"pipelineA"},
				Branch:    "featureA",
			},
		},
		want: "/blue/rest/organizations/jenkins/pipelines/pipelineA/branches/featureA/runs/",
	}, {
		name: "Get `Build` API with branch to be escaped",
		fields: fields{
			Organization: "jenkins",
		},
		args: args{
			BuildOption{
				Pipelines: []string{"pipelineA"},
				Branch:    "feature/a",
			},
		},
		want: "/blue/rest/organizations/jenkins/pipelines/pipelineA/branches/feature%2Fa/runs/",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jenkinsCore := core.JenkinsCore{}
			c := &BlueOceanClient{
				JenkinsCore:  jenkinsCore,
				Organization: tt.fields.Organization,
			}
			if got := c.getBuildAPI(tt.args.option); got != tt.want {
				t.Errorf("getBuildAPI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlueOceanClient_getGetBuildAPI(t *testing.T) {
	type fields struct {
		Organization string
	}
	type args struct {
		option GetBuildOption
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{{
		name: "Get `GetBuild` API",
		fields: fields{
			Organization: "jenkins",
		},
		args: args{
			GetBuildOption{
				Pipelines: []string{"pipelineA"},
				RunID:     "123",
			},
		},
		want: "/blue/rest/organizations/jenkins/pipelines/pipelineA/runs/123/",
	}, {
		name: "Get `GetBuild` API with branch",
		fields: fields{
			Organization: "jenkins",
		},
		args: args{
			GetBuildOption{
				Pipelines: []string{"pipelineA"},
				Branch:    "featureA",
				RunID:     "123",
			},
		},
		want: "/blue/rest/organizations/jenkins/pipelines/pipelineA/branches/featureA/runs/123/",
	}, {
		name: "Get `GetBuild` API with branch to be escaped",
		fields: fields{
			Organization: "jenkins",
		},
		args: args{
			GetBuildOption{
				Pipelines: []string{"pipelineA"},
				Branch:    "feature/a",
				RunID:     "123",
			},
		},
		want: "/blue/rest/organizations/jenkins/pipelines/pipelineA/branches/feature%2Fa/runs/123/",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &BlueOceanClient{
				JenkinsCore:  core.JenkinsCore{},
				Organization: tt.fields.Organization,
			}
			if got := c.getGetBuildAPI(tt.args.option); got != tt.want {
				t.Errorf("getGetBuildAPI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlueOceanClient_getGetNodesAPI(t *testing.T) {
	type args struct {
		option GetNodesOption
	}
	tests := []struct {
		name string
		args args
		want string
	}{{
		name: "Option without limit",
		args: args{
			option: GetNodesOption{
				Pipelines: []string{"pipelineA"},
				Branch:    "main",
				RunID:     "123",
			},
		},
		want: "/blue/rest/organizations/jenkins/pipelines/pipelineA/branches/main/runs/123/nodes/?limit=10000",
	}, {
		name: "Option with limit",
		args: args{
			option: GetNodesOption{
				Pipelines: []string{"pipelineA"},
				Branch:    "main",
				RunID:     "123",
				Limit:     456,
			},
		},
		want: "/blue/rest/organizations/jenkins/pipelines/pipelineA/branches/main/runs/123/nodes/?limit=456",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &BlueOceanClient{
				JenkinsCore:  core.JenkinsCore{},
				Organization: "jenkins",
			}
			if got := c.getGetNodesAPI(tt.args.option); got != tt.want {
				t.Errorf("getGetNodesAPI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlueOceanClient_getReplayAPI(t *testing.T) {
	type args struct {
		option *ReplayOption
	}
	tests := []struct {
		name string
		args args
		want string
	}{{
		name: "Multi-branch PipelineRun replay",
		args: args{
			option: &ReplayOption{
				Folders: []string{"pipelineA"},
				Branch:  "main",
				RunID:   "123",
			},
		},
		want: "/blue/rest/organizations/jenkins/pipelines/pipelineA/branches/main/runs/123/replay/",
	}, {
		name: "Normal PipelineRun replay",
		args: args{
			option: &ReplayOption{
				Folders: []string{"pipelineA"},
				RunID:   "123",
			},
		},
		want: "/blue/rest/organizations/jenkins/pipelines/pipelineA/runs/123/replay/",
	}, {
		name: "Multi-folder PipelineRun replay",
		args: args{
			option: &ReplayOption{
				Folders: []string{"folderA", "pipelineA"},
				RunID:   "123",
			},
		},
		want: "/blue/rest/organizations/jenkins/pipelines/folderA/pipelines/pipelineA/runs/123/replay/",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &BlueOceanClient{
				JenkinsCore:  core.JenkinsCore{},
				Organization: "jenkins",
			}
			if got := c.getReplayAPI(tt.args.option); got != tt.want {
				t.Errorf("BlueOceanClient.getReplayAPI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlueOceanClient_getGetPipelineAPI(t *testing.T) {
	type args struct {
		pipelineName string
		folders      []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{{
		name: "Without folder",
		args: args{
			pipelineName: "pipeline1",
		},
		want: "/blue/rest/organizations/jenkins/pipelines/pipeline1",
	}, {
		name: "Single folder",
		args: args{
			pipelineName: "pipeline1",
			folders:      []string{"folder1"},
		},
		want: "/blue/rest/organizations/jenkins/pipelines/folder1/pipelines/pipeline1",
	}, {
		name: "Tweo folders",
		args: args{
			pipelineName: "pipeline1",
			folders:      []string{"folder1", "folder2"},
		},
		want: "/blue/rest/organizations/jenkins/pipelines/folder1/pipelines/folder2/pipelines/pipeline1",
	}, {
		name: "Three folders",
		args: args{
			pipelineName: "pipeline1",
			folders:      []string{"folder1", "folder2", "folder3"},
		},
		want: "/blue/rest/organizations/jenkins/pipelines/folder1/pipelines/folder2/pipelines/folder3/pipelines/pipeline1",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &BlueOceanClient{
				JenkinsCore:  core.JenkinsCore{},
				Organization: "jenkins",
			}
			if got := c.getGetPipelineAPI(tt.args.pipelineName, tt.args.folders...); got != tt.want {
				t.Errorf("BlueOceanClient.getGetPipelineAPI() = %v, want %v", got, tt.want)
			}
		})
	}
}
