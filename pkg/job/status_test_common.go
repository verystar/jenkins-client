package job

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/verystar/jenkins-client/pkg/core"

	"github.com/verystar/jenkins-client/pkg/mock/mhttp"
)

// PrepareGetStatus only for test
func PrepareGetStatus(roundTripper *mhttp.MockRoundTripper, rootURL, user, password string) {
	request, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/json", rootURL), nil)
	response := &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
		Request:    request,
		Body:       ioutil.NopCloser(bytes.NewBufferString(`{"nodeName":"master"}`)),
	}
	response.Header.Add("X-Jenkins", "version")
	roundTripper.EXPECT().
		RoundTrip(core.NewRequestMatcher(request)).Return(response, nil)

	if user != "" && password != "" {
		request.SetBasicAuth(user, password)
	}
}
