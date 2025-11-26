package extauthzserver

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	envoy_service_auth_v3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/go-logr/logr"
	"google.golang.org/genproto/googleapis/rpc/code"
	googlestatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	log   logr.Logger
	store *store
}

var _ envoy_service_auth_v3.AuthorizationServer = &server{}

// New creates a new authorization server.
func New(log logr.Logger, dir fs.FS) (envoy_service_auth_v3.AuthorizationServer, error) {
	store, err := newStore(dir)
	if err != nil {
		return nil, fmt.Errorf("setting up store: %w", err)
	}
	return &server{
		log:   log,
		store: store,
	}, nil
}

// Check implements authorization's Check interface which performs authorization check based on the
// attributes associated with the incoming request.
func (s *server) Check(
	ctx context.Context,
	req *envoy_service_auth_v3.CheckRequest,
) (*envoy_service_auth_v3.CheckResponse, error) {
	if req.Attributes == nil || req.Attributes.Request == nil || req.Attributes.Request.Http == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	http := req.Attributes.Request.Http
	auth := http.Headers["Authorization"]
	if auth == "" {
		return nil, status.Error(codes.InvalidArgument, "missing Authorization header")
	}
	host := req.Attributes.Request.Http.Host
	hostParts := strings.Split(host, ".")
	if len(hostParts) == 0 || host == "" {
		s.log.Info("[WARN] no Host header in request found, denying request")
		return nil, status.Error(codes.InvalidArgument, "missing host")
	}

	err := s.store.IsValid(hostParts[0], []byte(auth))
	if err != nil {
		s.log.Error(err, "denied request ", "auth", auth)
		return &envoy_service_auth_v3.CheckResponse{
			Status: &googlestatus.Status{
				Code:    int32(code.Code_PERMISSION_DENIED),
				Message: "invalid authorization",
			},
		}, nil
	}

	return &envoy_service_auth_v3.CheckResponse{
		Status: &googlestatus.Status{
			Code: int32(code.Code_OK),
		},
	}, nil
}
