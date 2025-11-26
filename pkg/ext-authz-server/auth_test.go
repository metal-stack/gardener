package extauthzserver

import (
	"encoding/base64"
	"fmt"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("GRPC", func() {
	var s *server
	var password string
	BeforeEach(func() {
		password = "mypassword"
		hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		Expect(err).NotTo(HaveOccurred())
		s = &server{
			log: logr.Discard(),
			store: &store{
				map[string]auth{
					"app": {
						username:       "myuser",
						hashedPassword: hashed,
					},
				},
			},
		}
	})
	Describe("Check", func() {
		It("should return ok if hostname and basic auth matches", func(ctx SpecContext) {
			req := &authv3.CheckRequest{
				Attributes: &authv3.AttributeContext{
					Request: &authv3.AttributeContext_Request{
						Http: &authv3.AttributeContext_HttpRequest{
							Headers: map[string]string{
								"Authorization": fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "myuser:%s", password))),
							},
							Host: "app",
						},
					},
				},
			}
			resp, err := s.Check(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Status.Code).To(Equal(int32(code.Code_OK)))
		})
		It("should return denied if hostname does not match", func(ctx SpecContext) {
			req := &authv3.CheckRequest{
				Attributes: &authv3.AttributeContext{
					Request: &authv3.AttributeContext_Request{
						Http: &authv3.AttributeContext_HttpRequest{
							Headers: map[string]string{
								"Authorization": fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "myuser:%s", password))),
							},
							Host: "anyotherhost",
						},
					},
				},
			}
			resp, err := s.Check(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Status.Code).To(Equal(int32(code.Code_PERMISSION_DENIED)))
		})

		It("should return invalid argument error if Authorization header is missing", func(ctx SpecContext) {
			req := &authv3.CheckRequest{
				Attributes: &authv3.AttributeContext{
					Request: &authv3.AttributeContext_Request{
						Http: &authv3.AttributeContext_HttpRequest{
							Headers: map[string]string{},
							Host:    "app",
						},
					},
				},
			}
			_, err := s.Check(ctx, req)
			Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
		})

		It("should return invalid argument error if host is missing", func(ctx SpecContext) {
			req := &authv3.CheckRequest{
				Attributes: &authv3.AttributeContext{
					Request: &authv3.AttributeContext_Request{
						Http: &authv3.AttributeContext_HttpRequest{
							Headers: map[string]string{
								"Authorization": "any",
							},
						},
					},
				},
			}
			_, err := s.Check(ctx, req)
			Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
		})
	})
})
