#!/usr/bin/env bash
# Generate OIDC signing keys for local development (FR-110)
# Production deployments MUST use secure key vault / secrets manager

set -euo pipefail

KEYS_DIR="cmd/gridapi/internal/auth/keys"

echo "Generating OIDC signing keys for local development..."
echo ""
echo "⚠️  WARNING: These keys are for LOCAL DEVELOPMENT ONLY"
echo "   Production must load keys from secure key vault/secrets manager"
echo ""

# Create keys directory if it doesn't exist
mkdir -p "$KEYS_DIR"

# Generate ED25519 key pair (recommended for OIDC)
echo "Generating ED25519 key pair..."
ssh-keygen -t ed25519 -f "$KEYS_DIR/oidc_ed25519" -N "" -C "grid-oidc-dev" >/dev/null 2>&1

# Convert SSH public key to PEM format for JWKS
ssh-keygen -f "$KEYS_DIR/oidc_ed25519.pub" -e -m PEM > "$KEYS_DIR/oidc_ed25519_public.pem" 2>/dev/null

# Generate RSA key pair (fallback for compatibility)
echo "Generating RSA-2048 key pair..."
ssh-keygen -t rsa -b 2048 -f "$KEYS_DIR/oidc_rsa" -N "" -C "grid-oidc-dev-rsa" >/dev/null 2>&1

# Convert SSH public key to PEM format for JWKS
ssh-keygen -f "$KEYS_DIR/oidc_rsa.pub" -e -m PEM > "$KEYS_DIR/oidc_rsa_public.pem" 2>/dev/null

echo "✓ Keys generated successfully:"
echo "  - $KEYS_DIR/oidc_ed25519 (private)"
echo "  - $KEYS_DIR/oidc_ed25519.pub (public)"
echo "  - $KEYS_DIR/oidc_ed25519_public.pem (public PEM)"
echo "  - $KEYS_DIR/oidc_rsa (private)"
echo "  - $KEYS_DIR/oidc_rsa.pub (public)"
echo "  - $KEYS_DIR/oidc_rsa_public.pem (public PEM)"
echo ""
echo "Environment variable setup:"
echo "  export OIDC_SIGNING_KEY_PATH=$KEYS_DIR/oidc_ed25519"
echo "  export OIDC_SIGNING_KEY_ID=grid-dev-ed25519"
echo ""
echo "Production guidance:"
echo "  - Use AWS Secrets Manager, Azure Key Vault, or HashiCorp Vault"
echo "  - Rotate keys regularly (e.g., every 90 days)"
echo "  - Never commit private keys to git (.gitignore added)"
echo ""

# Add to .gitignore if not already present
if ! grep -q "cmd/gridapi/internal/auth/keys" .gitignore 2>/dev/null; then
    echo "cmd/gridapi/internal/auth/keys/" >> .gitignore
    echo "✓ Added keys directory to .gitignore"
fi

echo "✓ Setup complete!"
