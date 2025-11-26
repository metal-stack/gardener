package extauthzserver

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestExtAuthzServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ext Authz Server Bootstrappers Suite")
}
