package github_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestJenkinsClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "github client test")
}
