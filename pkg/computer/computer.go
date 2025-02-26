package computer

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"github.com/verystar/jenkins-client/pkg/core"
)

// Client is client for operate computers
type Client struct {
	core.JenkinsCore
}

// List get the computer list
func (c *Client) List() (computers List, err error) {
	err = c.RequestWithData(http.MethodGet, "/computer/api/json",
		nil, nil, 200, &computers)
	return
}

// Launch starts up a agent
func (c *Client) Launch(name string) (err error) {
	api := fmt.Sprintf("/computer/%s/launchSlaveAgent", name)
	_, err = c.RequestWithoutData(http.MethodPost, api, nil, nil, 200)
	return
}

// Delete removes a agent from Jenkins
func (c *Client) Delete(name string) (err error) {
	api := fmt.Sprintf("/computer/%s/doDelete", name)
	_, err = c.RequestWithoutData(http.MethodPost, api, nil, nil, 200)
	return
}

type agentJNLP struct {
	XMLName      xml.Name `xml:"jnlp"`
	AppArguments []string `xml:"application-desc>argument"`
}

// GetSecret returns the secret of an agent
func (c *Client) GetSecret(name string) (secret string, err error) {
	api := fmt.Sprintf("/computer/%s/slave-agent.jnlp", name)
	var response *http.Response
	if response, err = c.RequestWithResponse(http.MethodGet, api, nil, nil); err == nil {
		if response.StatusCode == http.StatusOK {
			var data []byte
			if data, err = ioutil.ReadAll(response.Body); err == nil {
				jnlp := &agentJNLP{}
				if err = xml.Unmarshal(data, jnlp); err == nil {
					secret = jnlp.AppArguments[0]
				} else {
					err = fmt.Errorf("invalid jnlp xml, error: %v", err)
				}
			}
		} else {
			err = fmt.Errorf("unexpected status code: %d", response.StatusCode)
		}
	}
	return
}

// GetLog fetch the log a computer
func (c *Client) GetLog(name string) (log string, err error) {
	var response *http.Response
	api := fmt.Sprintf("/computer/%s/logText/progressiveText", name)
	if response, err = c.RequestWithResponse(http.MethodGet, api, nil, nil); err == nil {
		statusCode := response.StatusCode
		if statusCode != 200 {
			err = fmt.Errorf("unexpected status code %d", statusCode)
			return
		}

		var data []byte
		if data, err = ioutil.ReadAll(response.Body); err == nil {
			log = string(data)
		}
	}
	return
}

// Create creates a computer by name
func (c *Client) Create(name string) (err error) {
	formData := url.Values{
		"name": {name},
		"mode": {"hudson.slaves.DumbSlave"},
	}
	payload := strings.NewReader(formData.Encode())
	if _, err = c.RequestWithoutData(http.MethodPost, "/computer/createItem",
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, payload, 200); err == nil {
		payload = GetPayloadForCreateAgent(name)
		_, err = c.RequestWithoutData(http.MethodPost, "/computer/doCreateItem",
			map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, payload, 200)
	}
	return
}

func getDefaultAgentLabels() string {
	return fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH)
}

// GetDefaultAgentWorkDir returns the Jenkins agent work dir
func GetDefaultAgentWorkDir() string {
	// TODO return different directory base on the OS
	return "/var/tmp/jenkins"
}

// GetPayloadForCreateAgent returns a payload for creating an agent
func GetPayloadForCreateAgent(name string) *strings.Reader {
	palyLoad := fmt.Sprintf(`{
	"name": "%s",
	"nodeDescription": "",
	"numExecutors": "1",
	"remoteFS": "%s",
	"labelString": "%s",
	"mode": "NORMAL",
	"launcher": {
		"$class": "hudson.slaves.JNLPLauncher",
		"workDirSettings": {
			"disabled": false,
			"workDirPath": "",
			"internalDir": "remoting",
			"failIfWorkDirIsMissing": false
		},
		"tunnel": "",
		"vmargs": ""
	},
	"type": "hudson.slaves.DumbSlave"
}`, name, GetDefaultAgentWorkDir(), getDefaultAgentLabels())
	formData := url.Values{
		"name": {name},
		"type": {"hudson.slaves.DumbSlave"},
		"json": {palyLoad},
	}
	return strings.NewReader(formData.Encode())
}

// Computer is the agent of Jenkins
type Computer struct {
	AssignedLabels      []Label
	Description         string
	DisplayName         string
	Idle                bool
	JnlpAgent           bool
	LaunchSupported     bool
	ManualLaunchAllowed bool
	NumExecutors        int
	Offline             bool
	OfflineCause        OfflineCause
	OfflineCauseReason  string
	TemporarilyOffline  bool
}

// OfflineCause is the cause of computer offline
type OfflineCause struct {
	Timestamp   int64
	Description string
}

// List represents the list of computer from API
type List struct {
	// nolint
	busyExecutors  int
	Computer       []Computer
	TotalExecutors int
}

// Label represents the label of a computer
type Label struct {
	Name string
}
