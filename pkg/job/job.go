package job

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/verystar/jenkins-client/pkg/core"
)

const (
	// StringParameterDefinition is the definition for string parameter
	StringParameterDefinition = "StringParameterDefinition"
	// FileParameterDefinition is the definition for file parameter
	FileParameterDefinition = "FileParameterDefinition"
)

// Client is client for operate jobs
type Client struct {
	core.JenkinsCore

	Parent string
}

// Search find a set of jobs by name
func (q *Client) Search(name, kind string, start, limit int) (items []JenkinsItem, err error) {
	err = q.RequestWithData(http.MethodGet, fmt.Sprintf("/items/list?name=%s&type=%s&start=%d&limit=%d&parent=%s",
		name, kind, start, limit, q.Parent),
		nil, nil, 200, &items)
	return
}

// SearchViaBlue searches jobs via the BlueOcean API
//
// Deprecated: For clearer client of BlueOcean, please use BlueOceanClient#Search instead
func (q *Client) SearchViaBlue(name string, start, limit int) (items []JenkinsItem, err error) {
	boClient := BlueOceanClient{JenkinsCore: q.JenkinsCore, Organization: "jenkins"}
	return boClient.Search(name, start, limit)
}

// Build trigger a job
func (q *Client) Build(jobName string) (err error) {
	path := ParseJobPath(jobName)
	_, err = q.RequestWithoutData(http.MethodPost, fmt.Sprintf("%s/build", path), nil, nil, 201)
	return
}

// IdentityBuild is the build which carry the identity cause
type IdentityBuild struct {
	Build Build
	Cause IdentityCause
}

// IdentityCause carray a identity cause
type IdentityCause struct {
	UUID             string `json:"uuid"`
	ShortDescription string `json:"shortDescription"`
	Message          string
}

// BuildAndReturn trigger a job then returns the build info
func (q *Client) BuildAndReturn(jobName, cause string, timeout, delay int) (build IdentityBuild, err error) {
	path := ParseJobPath(jobName)

	api := fmt.Sprintf("%s/restFul/build?1=1", path)
	if timeout >= 0 {
		api += fmt.Sprintf("&timeout=%d", timeout)
	}
	if delay >= 0 {
		api += fmt.Sprintf("&delay=%d", delay)
	}
	if cause != "" {
		api += fmt.Sprintf("&identifyCause=%s", cause)
	}

	err = q.RequestWithData(http.MethodPost, api, nil, nil, 200, &build)
	return
}

// GetBuild get build information of a job
func (q *Client) GetBuild(jobName string, id int) (job *Build, err error) {
	path := ParseJobPath(jobName)
	var api string
	if id == -1 {
		api = fmt.Sprintf("%s/lastBuild/api/json", path)
	} else {
		api = fmt.Sprintf("%s/%d/api/json", path, id)
	}

	err = q.RequestWithData("GET", api, nil, nil, 200, &job)
	return
}

// BuildWithParams build a job which has params
func (q *Client) BuildWithParams(jobName string, parameters []ParameterDefinition) (err error) {
	path := ParseJobPath(jobName)
	api := fmt.Sprintf("%s/build", path)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	defer func(writer *multipart.Writer) {
		// ignore error
		_ = writer.Close()
	}(writer)

	hasFileParam := false
	stringParameters := make([]ParameterDefinition, 0, len(parameters))
	for _, parameter := range parameters {
		if parameter.Type == FileParameterDefinition {
			hasFileParam = true
			var file *os.File
			file, err = os.Open(parameter.Filepath)
			if err != nil {
				return err
			}
			defer func(file *os.File) {
				// ignore error
				_ = file.Close()
			}(file)

			var fWriter io.Writer
			fWriter, err = writer.CreateFormFile(parameter.Filepath, filepath.Base(parameter.Filepath))
			if err != nil {
				return err
			}
			_, err = io.Copy(fWriter, file)
		} else {
			stringParameters = append(stringParameters, parameter)
		}
	}

	var paramJSON []byte
	if len(stringParameters) == 1 {
		paramJSON, err = json.Marshal(stringParameters[0])
	} else {
		paramJSON, err = json.Marshal(stringParameters)
	}
	if err != nil {
		return
	}

	if hasFileParam {
		if err = writer.WriteField("json", fmt.Sprintf("{\"parameter\": %s}", string(paramJSON))); err != nil {
			return
		}

		if err = writer.Close(); err != nil {
			return
		}

		_, err = q.RequestWithoutData(http.MethodPost, api,
			map[string]string{"Content-Type": writer.FormDataContentType()}, body, 201)
	} else {
		formData := url.Values{"json": {fmt.Sprintf("{\"parameter\": %s}", string(paramJSON))}}
		payload := strings.NewReader(formData.Encode())

		_, err = q.RequestWithoutData(http.MethodPost, api,
			map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, payload, 201)
	}
	return
}

// DisableJob disable a job
func (q *Client) DisableJob(jobName string) (err error) {
	path := ParseJobPath(jobName)
	api := fmt.Sprintf("%s/disable", path)

	_, err = q.RequestWithoutData(http.MethodPost, api, nil, nil, 200)
	return
}

// EnableJob disable a job
func (q *Client) EnableJob(jobName string) (err error) {
	path := ParseJobPath(jobName)
	api := fmt.Sprintf("%s/enable", path)

	_, err = q.RequestWithoutData(http.MethodPost, api, nil, nil, 200)
	return
}

// StopJob stops a job build
func (q *Client) StopJob(jobName string, num int) (err error) {
	path := ParseJobPath(jobName)

	var api string
	if num <= 0 {
		api = fmt.Sprintf("%s/lastBuild/stop", path)
	} else {
		api = fmt.Sprintf("%s/%d/stop", path, num)
	}

	_, err = q.RequestWithoutData(http.MethodPost, api, nil, nil, 200)
	return
}

// GetJob returns the job info
func (q *Client) GetJob(name string) (job *Job, err error) {
	path := ParseJobPath(name)
	api := fmt.Sprintf("%s/api/json", path)

	err = q.RequestWithData(http.MethodGet, api, nil, nil, 200, &job)
	return
}

// AddParameters add parameters to a SimplePipeline
func (q *Client) AddParameters(name, parameters string) (err error) {
	path := ParseJobPath(name)
	api := fmt.Sprintf("%s/restFul/addParameter", path)

	formData := url.Values{
		"params": {parameters},
	}
	payload := strings.NewReader(formData.Encode())
	_, err = q.RequestWithoutData(http.MethodPost, api, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, payload, 200)
	return
}

// RemoveParameters add parameters to a SimplePipeline
func (q *Client) RemoveParameters(name, parameters string) (err error) {
	path := ParseJobPath(name)
	api := fmt.Sprintf("%s/restFul/removeParameter?params=%s", path, parameters)

	_, err = q.RequestWithoutData(http.MethodPost, api, nil, nil, 200)
	return
}

// GetJobTypeCategories returns all categories of jobs
func (q *Client) GetJobTypeCategories() (jobCategories []Category, err error) {
	var (
		statusCode int
		data       []byte
	)

	if statusCode, data, err = q.Request("GET", "/view/all/itemCategories?depth=3", nil, nil); err == nil {
		if statusCode == 200 {
			type innerJobCategories struct {
				Categories []Category
			}
			result := &innerJobCategories{}
			err = json.Unmarshal(data, result)
			jobCategories = result.Categories
		} else {
			err = fmt.Errorf("unexpected status code: %d", statusCode)
		}
	}
	return
}

// GetPipeline return the pipeline object
func (q *Client) GetPipeline(name string) (pipeline *SimplePipeline, err error) {
	path := ParseJobPath(name)
	api := fmt.Sprintf("%s/restFul", path)
	err = q.RequestWithData("GET", api, nil, nil, 200, &pipeline)
	return
}

// UpdatePipeline updates the pipeline script
func (q *Client) UpdatePipeline(name, script string) (err error) {
	formData := url.Values{}
	formData.Add("script", script)

	path := ParseJobPath(name)
	api := fmt.Sprintf("%s/restFul/update?%s", path, formData.Encode())

	_, err = q.RequestWithoutData(http.MethodPost, api, nil, nil, 200)
	return
}

// GetHistory returns the build history of a job
func (q *Client) GetHistory(name string) (builds []*Build, err error) {
	var job *Job
	if job, err = q.GetJob(name); err == nil {
		buildList := job.Builds // only contains basic info

		var build *Build
		for _, buildItem := range buildList {
			build, err = q.GetBuild(name, buildItem.Number)
			if err != nil {
				break
			}
			builds = append(builds, build)
		}
	}
	return
}

// DeleteHistory returns the build history of a job
func (q *Client) DeleteHistory(jobName string, num int) (err error) {
	path := ParseJobPath(jobName)
	api := fmt.Sprintf("%s/%d/doDelete", path, num)
	_, err = q.RequestWithoutData(http.MethodPost, api, nil, nil, 200)
	return
}

// Log get the log of a job
func (q *Client) Log(jobName string, history int, start int64) (jobLog Log, err error) {
	path := ParseJobPath(jobName)
	var api string
	if history == -1 {
		api = fmt.Sprintf("%s%s/lastBuild/logText/progressiveText?start=%d", q.URL, path, start)
	} else {
		api = fmt.Sprintf("%s%s/%d/logText/progressiveText?start=%d", q.URL, path, history, start)
	}
	var (
		req      *http.Request
		response *http.Response
	)

	req, err = http.NewRequest("GET", api, nil)
	if err == nil {
		err = q.AuthHandle(req)
	}
	if err != nil {
		return
	}

	client := q.GetClient()
	jobLog = Log{
		HasMore:   false,
		Text:      "",
		NextStart: int64(0),
	}

	if response, err = client.Do(req); err == nil {
		code := response.StatusCode
		var data []byte
		data, err = io.ReadAll(response.Body)
		if code == 200 {
			jobLog.Text = string(data)

			if response.Header != nil {
				jobLog.HasMore = strings.ToLower(response.Header.Get("X-More-Data")) == "true"
				jobLog.NextStart, _ = strconv.ParseInt(response.Header.Get("X-Text-Size"), 10, 64)
			}
		}
	}
	return
}

// CreateJobPayload the payload for creating a job
type CreateJobPayload struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
	From string `json:"from"`
}

// Create can create a job
func (q *Client) Create(jobPayload CreateJobPayload) (err error) {
	return q.CreateJobInFolder(jobPayload, "")
}

// CreateJobInFolder creates a job in a specific folder and create folder first if the folder does not exist
func (q *Client) CreateJobInFolder(jobPayload CreateJobPayload, path string) (err error) {
	// create a job in path
	playLoadData, _ := json.Marshal(jobPayload)
	formData := url.Values{
		"json": {string(playLoadData)},
		"name": {jobPayload.Name},
		"mode": {jobPayload.Mode},
		"from": {jobPayload.From},
	}
	payload := strings.NewReader(formData.Encode())
	path = ParseJobPath(path)
	api := fmt.Sprintf("/view/all%s/createItem", path)
	var code int
	code, err = q.RequestWithoutData(http.MethodPost, api,
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, payload, 200)
	if code == 302 {
		err = nil
	}
	return
}

// Delete will delete a job by name
func (q *Client) Delete(jobName string) (err error) {
	var (
		statusCode int
	)

	jobName = ParseJobPath(jobName)
	api := fmt.Sprintf("%s/doDelete", jobName)
	header := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	if statusCode, _, err = q.Request(http.MethodPost, api, header, nil); err == nil {
		if statusCode != 200 && statusCode != 302 {
			err = fmt.Errorf("unexpected status code: %d", statusCode)
		}
	}
	return
}

// GetJobInputActions returns the all pending actions
func (q *Client) GetJobInputActions(jobName string, buildID int) (actions []InputItem, err error) {
	path := ParseJobPath(jobName)
	err = q.RequestWithData("GET", fmt.Sprintf("%s/%d/wfapi/pendingInputActions", path, buildID), nil, nil, 200, &actions)
	return
}

// JenkinsInputParametersRequest represents the parameters for the Jenkins input request
type JenkinsInputParametersRequest struct {
	Parameter []ParameterDefinition `json:"parameter"`
}

// JobInputSubmit submit the pending input request
func (q *Client) JobInputSubmit(jobName, inputID string, buildID int, abort bool, params map[string]string) (err error) {
	jobPath := ParseJobPath(jobName)
	var api string
	if abort {
		api = fmt.Sprintf("%s/%d/input/%s/abort", jobPath, buildID, inputID)
	} else {
		api = fmt.Sprintf("%s/%d/input/%s/proceed", jobPath, buildID, inputID)
	}

	request := JenkinsInputParametersRequest{
		Parameter: make([]ParameterDefinition, 0),
	}

	for k, v := range params {
		request.Parameter = append(request.Parameter, ParameterDefinition{
			Name:  k,
			Value: v,
		})
	}

	paramData, _ := json.Marshal(request)

	api = fmt.Sprintf("%s?json=%s", api, string(paramData))
	_, err = q.RequestWithoutData(http.MethodPost, api, nil, nil, 200)

	return
}

// ParseJobPath leads with slash
func ParseJobPath(jobName string) (path string) {
	path = jobName
	if jobName == "" || strings.HasPrefix(jobName, "/job/") ||
		strings.HasPrefix(jobName, "job/") {
		return
	}

	jobItems := strings.Split(jobName, " ")
	path = ""
	for _, item := range jobItems {
		path = fmt.Sprintf("%s/job/%s", path, item)
	}
	return
}

// parsePipelinePath parses multiple pipelines and leads with slash.
// e.g.: pipelines/a/pipelines/b
func parsePipelinePath(pipelines []string) string {
	if len(pipelines) == 0 {
		return ""
	}
	return "pipelines/" + strings.Join(pipelines, "/pipelines/")
}

// Log holds the log text
type Log struct {
	HasMore   bool
	NextStart int64
	Text      string
}

// JenkinsItem represents the item of Jenkins
type JenkinsItem struct {
	Name        string
	DisplayName string
	URL         string
	Description string
	Type        string

	/** comes from Job */
	Buildable bool
	Building  bool
	InQueue   bool

	/** comes from ParameterizedJob */
	Parameterized bool
	Disabled      bool

	/** comes from blueOcean */
	FullName     string
	WeatherScore int
	Parameters   []ParameterDefinition
}

// Job represents a job
type Job struct {
	Type            string `json:"_class"`
	Builds          []Build
	Color           string
	ConcurrentBuild bool
	Name            string
	NextBuildNumber int
	URL             string
	Buildable       bool

	Property []ParametersDefinitionProperty
}

// ParametersDefinitionProperty holds the param definition property
type ParametersDefinitionProperty struct {
	ParameterDefinitions []ParameterDefinition
}

// ParameterDefinition holds the parameter definition
// Reference: https://github.com/jenkinsci/jenkins/blob/master/core/src/main/java/hudson/model/ParameterDefinition.java#L98
type ParameterDefinition struct {
	Description           string          `json:"description,omitempty"`
	Name                  string          `json:"name"`
	Type                  string          `json:"type"`
	Value                 string          `json:"value"`
	Filepath              string          `json:"file,omitempty"`
	DefaultParameterValue *ParameterValue `json:"defaultParameterValue,omitempty"`

	// Reference: https://github.com/jenkinsci/jenkins/blob/65b9f1cf51c3b3cf44ecb7d51d3f30d7dbe6b3bd/core/src/main/java/hudson/model/RunParameterDefinition.java#L103-L106
	Choices []string `json:"choices,omitempty"`

	// Reference: https://github.com/jenkinsci/jenkins/blob/65b9f1cf51c3b3cf44ecb7d51d3f30d7dbe6b3bd/core/src/main/java/hudson/model/RunParameterDefinition.java#L103-L106
	ProjectName string `json:"projectName,omitempty"`
	// Reference: https://github.com/jenkinsci/jenkins/blob/65b9f1cf51c3b3cf44ecb7d51d3f30d7dbe6b3bd/core/src/main/java/hudson/model/RunParameterDefinition.java#L116-L121
	Filter string `json:"filter,omitempty"`
}

// ParameterValue represents the value for param
// Reference: https://github.com/jenkinsci/jenkins/blob/f23512f2bc97d18cd4f0183a7db4a62bc6b84196/core/src/main/java/hudson/model/ParameterValue.java#L78
type ParameterValue struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value,omitempty"`

	// Fields of RunParameterValue
	// Reference: https://github.com/jenkinsci/jenkins/blob/f23512f2bc97d18cd4f0183a7db4a62bc6b84196/core/src/main/java/hudson/model/RunParameterValue.java#L33
	JobName string `json:"jobName,omitempty"`
	Number  string `json:"number,omitempty"`
}

// SimpleJobBuild represents a simple job build
type SimpleJobBuild struct {
	Number int
	URL    string
}

// Build represents a job build
type Build struct {
	SimpleJobBuild
	Building          bool
	Description       string
	DisplayName       string
	Duration          int64
	EstimatedDuration int64
	FullDisplayName   string
	ID                string
	KeepLog           bool
	QueueID           int
	Result            string
	Timestamp         int64
	PreviousBuild     SimpleJobBuild
	NextBuild         SimpleJobBuild
}

// SimplePipeline represents a pipeline
type SimplePipeline struct {
	Script  string
	Sandbox bool
}

// Category represents a job category
type Category struct {
	Description string
	ID          string
	Items       []CategoryItem
	MinToShow   int
	Name        string
	Order       int
}

// CategoryItem represents a job category item
type CategoryItem struct {
	Description string
	DisplayName string
	Order       int
	Class       string
}

// InputItem represents a job input action
type InputItem struct {
	ID                  string
	AbortURL            string
	Message             string
	ProceedText         string
	ProceedURL          string
	RedirectApprovalURL string
	Inputs              []ParameterDefinition
}
