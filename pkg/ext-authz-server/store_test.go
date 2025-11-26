package extauthzserver

import (
	"testing/fstest"

	"github.com/gardener/gardener/pkg/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/bcrypt"
)

var _ = Describe("Store", func() {
	Describe("#readSecrets", func() {
		It("should parse files correctly", func() {
			testFs := fstest.MapFS{
				"app": &fstest.MapFile{
					Data: testCredentials("foo", "bar"),
				},
			}
			secrets, err := readSecrets(testFs)
			Expect(err).NotTo(HaveOccurred())
			Expect(secrets).To(HaveKey("app"))
			a := secrets["app"]
			Expect(a.username).To(Equal("foo"))
			Expect(bcrypt.CompareHashAndPassword(a.hashedPassword, []byte("bar"))).NotTo(HaveOccurred())
		})
		It("should parse error if file does not contain htpasswd format", func() {
			testFs := fstest.MapFS{
				"app": &fstest.MapFile{
					Data: []byte("blablablub"),
				},
			}
			_, err := readSecrets(testFs)
			Expect(err).To(MatchError(ContainSubstring("does not contain valid basic auth")))
		})
	})
})

func testCredentials(username, password string) []byte {
	GinkgoHelper()
	htpasswd, err := utils.CreateBcryptCredentials([]byte(username), []byte(password))
	Expect(err).NotTo(HaveOccurred())
	return htpasswd
}
