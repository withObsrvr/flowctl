//go:build tools

// Package tools pins dependencies that are required but invisible to `go mod tidy`.
//
// test/testdata/delivery_component.go is compiled at run time by the delivery tests
// (see delivery_process_test.go and direct_sink_routing_test.go), not by the Go build.
// The toolchain ignores testdata directories entirely, so tidy cannot see that import
// and would otherwise drop the module — breaking those tests with a confusing
// "no required module provides package" failure at test time rather than build time.
//
// Build constraints do not hide imports from `go mod tidy`, which is what makes this
// file work while keeping the import out of every real build.
package tools

import (
	_ "github.com/withObsrvr/flowctl-sdk/pkg/consumer"
)
