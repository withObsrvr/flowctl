package cliout

import (
	"context"
	"os"
)

type contextKey struct{}

// NewContext returns ctx carrying p, so subcommands in other packages can reach the
// Printer the root command built from the global --output flag without a package
// variable.
func NewContext(ctx context.Context, p *Printer) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

// FromContext returns the Printer in ctx.
//
// It never returns nil. A command invoked outside the normal root wiring — in a test,
// say — gets a plain table Printer on the real streams rather than a panic, which
// keeps cliout usable from a unit test with no setup.
func FromContext(ctx context.Context) *Printer {
	if ctx != nil {
		if p, ok := ctx.Value(contextKey{}).(*Printer); ok && p != nil {
			return p
		}
	}
	return New(FormatTable, os.Stdout, os.Stderr)
}
