package cmdutil

import (
	"fmt"

	"github.com/uptrace/bun"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/bunx"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/iam"
)

// IAMServiceOptions controls how the CLI constructs the IAM service.
type IAMServiceOptions struct {
	// EnableAutoSave mirrors the previous per-command enforcer toggles.
	EnableAutoSave bool
}

// IAMServiceBundle bundles the service with its underlying DB connection so callers can
// reuse the connection for other repositories when necessary.
type IAMServiceBundle struct {
	Service iam.Service
	DB      *bun.DB
}

// Close releases the underlying database connection.
func (b *IAMServiceBundle) Close() {
	if b == nil || b.DB == nil {
		return
	}
	bunx.Close(b.DB)
}

// NewIAMServiceBundle centralizes IAM service construction for CLI commands.
// It wires repositories, initializes Casbin, and returns a ready-to-use service.
func NewIAMServiceBundle(cfg *config.Config, opts IAMServiceOptions) (*IAMServiceBundle, error) {
	db, err := bunx.NewDB(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	enforcer, err := auth.InitEnforcer(db)
	if err != nil {
		bunx.Close(db)
		return nil, fmt.Errorf("failed to initialize casbin enforcer: %w", err)
	}
	enforcer.EnableAutoSave(opts.EnableAutoSave)

	deps := iam.IAMServiceDependencies{
		Users:           repository.NewBunUserRepository(db),
		ServiceAccounts: repository.NewBunServiceAccountRepository(db),
		Sessions:        repository.NewBunSessionRepository(db),
		UserRoles:       repository.NewBunUserRoleRepository(db),
		GroupRoles:      repository.NewBunGroupRoleRepository(db),
		Roles:           repository.NewBunRoleRepository(db),
		RevokedJTIs:     repository.NewBunRevokedJTIRepository(db),
		Enforcer:        enforcer,
	}

	iamService, err := iam.NewIAMService(deps, iam.IAMServiceConfig{Config: cfg})
	if err != nil {
		bunx.Close(db)
		return nil, fmt.Errorf("failed to create IAM service: %w", err)
	}

	return &IAMServiceBundle{
		Service: iamService,
		DB:      db,
	}, nil
}
