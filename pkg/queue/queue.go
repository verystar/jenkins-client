package queue

import (
	"fmt"
	"net/http"

	"github.com/verystar/jenkins-client/pkg/core"
)

// Client is the client of queue
type Client struct {
	core.JenkinsCore
}

// Get returns the job queue
func (q *Client) Get() (status *JobQueue, err error) {
	err = q.RequestWithData(http.MethodGet, "/queue/api/json", nil, nil, 200, &status)
	return
}

// Cancel will cancel a job from the queue
func (q *Client) Cancel(id int) (err error) {
	api := fmt.Sprintf("/queue/cancelItem?id=%d", id)
	var statusCode int
	if statusCode, err = q.RequestWithoutData(http.MethodPost, api, nil, nil, 302); err != nil &&
		(statusCode == 200 ||
			statusCode == 404) { // 404 should be an error, but no idea why it can be triggered successful
		err = nil
	}
	return
}

// JobQueue represent the job queue
type JobQueue struct {
	Items []Item
}

// Item is the item of job queue
type Item struct {
	Blocked                    bool
	Buildable                  bool
	ID                         int
	Params                     string
	Pending                    bool
	Stuck                      bool
	URL                        string
	Why                        string
	BuildableStartMilliseconds int64
	InQueueSince               int64
	Actions                    []CauseAction
}

// CauseAction is the collection of causes
type CauseAction struct {
	Causes []Cause
}

// Cause represent the reason why job is triggered
type Cause struct {
	UpstreamURL      string
	UpstreamProject  string
	UpstreamBuild    int
	ShortDescription string
}
