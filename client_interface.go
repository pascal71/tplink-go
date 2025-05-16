// Package client provides the interface definition for mocking.
package client

import (
	"context"
)

// Interface defines the minimal SSH interaction contract.
type Interface interface {
	Connect(ctx context.Context) error
	RunCommand(ctx context.Context, command string) (string, error)
	Close()
}
