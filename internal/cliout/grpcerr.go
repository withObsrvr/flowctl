package cliout

import (
	"context"

	"github.com/withobsrvr/flowctl/internal/exitcode"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnreachableHint turns a "cannot reach the control plane" failure into an
// error that says what to do about it.
//
// grpc.NewClient connects lazily, so a dead control plane no longer fails at
// dial time the way grpc.DialContext with WithBlock did — it fails on the first
// RPC, as a bare "connection refused". Without this, the single most common
// error an operator hits loses its hint.
func UnreachableHint(endpoint string, err error) error {
	if err == nil {
		return nil
	}
	if status.Code(err) != codes.Unavailable {
		return err
	}
	return exitcode.Errorf(exitcode.Unavailable,
		"no control plane is reachable at %s\n"+
			"start one with: flowctl run <pipeline.yaml>\n"+
			"or point at a different one with --control-plane-address/--control-plane-port",
		endpoint)
}

// UnreachableInterceptor applies UnreachableHint to every unary call on a
// connection, so each command does not have to remember to.
func UnreachableInterceptor(endpoint string) grpc.DialOption {
	return grpc.WithChainUnaryInterceptor(func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		return UnreachableHint(endpoint, invoker(ctx, method, req, reply, cc, opts...))
	})
}
