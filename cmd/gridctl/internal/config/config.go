package config

import (
	"context"

	"github.com/terraconstructs/grid/cmd/gridctl/internal/client"
)

type contextKey string

const configKey contextKey = "gridctl-config"

// GlobalConfig holds shared configuration for all gridctl commands.
// This is injected into the cobra command context by the root command's
// PersistentPreRun hook and consumed by all subcommands.
type GlobalConfig struct {
	ServerURL      string
	NonInteractive bool
	ClientProvider *client.Provider
}

// InjectConfig adds config to the cobra command context.
// This should be called in the root command's PersistentPreRun.
func InjectConfig(ctx context.Context, cfg *GlobalConfig) context.Context {
	return context.WithValue(ctx, configKey, cfg)
}

// FromContext retrieves config from the cobra command context.
// Returns (nil, false) if config is not present.
func FromContext(ctx context.Context) (*GlobalConfig, bool) {
	cfg, ok := ctx.Value(configKey).(*GlobalConfig)
	return cfg, ok
}

// MustFromContext retrieves config from context or panics.
// This should only be used in command RunE functions where we know
// the config has been injected by the root command.
func MustFromContext(ctx context.Context) *GlobalConfig {
	cfg, ok := FromContext(ctx)
	if !ok {
		panic("gridctl: config not found in context - this is a bug in gridctl")
	}
	return cfg
}
