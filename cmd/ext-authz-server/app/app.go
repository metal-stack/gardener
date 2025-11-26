// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"net"
	"os"

	envoy_service_auth_v3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/gardener/gardener/cmd/utils/initrun"
	extauthzserver "github.com/gardener/gardener/pkg/ext-authz-server"
)

// Name is a const for the name of this component.
const Name = "ext-authz-server"

// NewCommand creates a new cobra.Command for running gardenadm.
func NewCommand() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use: Name,

		RunE: func(cmd *cobra.Command, _ []string) error {
			log, err := initrun.InitRun(cmd, opts, Name)
			if err != nil {
				return err
			}
			if err := run(cmd.Context(), log, opts); err != nil {
				log.Error(err, "starting server")
			}
			return err
		},
	}

	opts.AddFlags(cmd.Flags())

	return cmd
}

func run(ctx context.Context, log logr.Logger, o *options) error {
	port := o.port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen to %d: %v", port, err)
	}

	gs := grpc.NewServer()

	if o.reflection {
		reflection.Register(gs)
	}
	authsrv, err := extauthzserver.New(log, os.DirFS(o.secretsDir))
	if err != nil {
		return fmt.Errorf("setting up authorization server: %w", err)
	}
	envoy_service_auth_v3.RegisterAuthorizationServer(gs, authsrv)

	log.Info("starting gRPC server", "port", port)

	errc := make(chan error)
	go func(errc chan error) {
		defer close(errc)
		errc <- gs.Serve(lis)
	}(errc)

	select {
	case <-ctx.Done():
		log.Info("graceful shutdown")
		gs.GracefulStop()
		return nil
	case err := <-errc:
		return err
	}
}
