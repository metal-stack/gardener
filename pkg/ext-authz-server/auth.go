package extauthzserver

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	envoycorev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoytypev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	envoy_service_auth_v3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/go-logr/logr"
	"google.golang.org/genproto/googleapis/rpc/code"
	googlestatus "google.golang.org/genproto/googleapis/rpc/status"
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
	s.log.Info("auth request")

	if req.Attributes == nil || req.Attributes.Request == nil || req.Attributes.Request.Http == nil {
		return denyResponse(s.log, "invalid request"), nil
	}
	http := req.Attributes.Request.Http

	log := s.log.WithValues("id", http.Id, "path", http.Path, "host", http.Host)

	auth := http.Headers["authorization"]
	if auth == "" {
		return denyResponse(log, "missing authorization header"), nil
	}
	host := req.Attributes.Request.Http.Host

	hostParts := strings.Split(host, ".")
	if len(hostParts) == 0 || host == "" {
		log.Info("[WARN] no Host header in request found, denying request")
		return denyResponse(log, "missing host"), nil
	}

	err := s.store.IsValid(hostParts[0], []byte(auth))
	if err != nil {
		log.Error(err, "denied request ", "auth", auth)
		return denyResponse(log, "invalid authorization"), nil
	}

	log.Info("auth request allowed", "host", host)

	return &envoy_service_auth_v3.CheckResponse{
		Status: &googlestatus.Status{
			Code: int32(code.Code_OK),
		},
		HttpResponse: &envoy_service_auth_v3.CheckResponse_OkResponse{
			OkResponse: &envoy_service_auth_v3.OkHttpResponse{
				HeadersToRemove: []string{"authorization"},
			},
		},
	}, nil
}

func denyResponse(log logr.Logger, message string) *envoy_service_auth_v3.CheckResponse {
	log.Info("auth denied", "message", message)

	missingAuthResponse := &envoy_service_auth_v3.CheckResponse_DeniedResponse{
		DeniedResponse: &envoy_service_auth_v3.DeniedHttpResponse{
			Headers: []*envoycorev3.HeaderValueOption{
				{
					Header: &envoycorev3.HeaderValue{Key: "WWW-Authenticate", Value: "Basic realm=\"Authentication Required\""},
				},
			},
			Status: &envoytypev3.HttpStatus{Code: envoytypev3.StatusCode_Unauthorized},
		},
	}
	return &envoy_service_auth_v3.CheckResponse{
		Status: &googlestatus.Status{
			Code:    int32(code.Code_PERMISSION_DENIED),
			Message: message,
		},
		HttpResponse: missingAuthResponse,
	}
}
