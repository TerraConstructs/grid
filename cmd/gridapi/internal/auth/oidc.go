package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

var (
	// ErrOIDCDisabled is returned when OIDC configuration is incomplete.
	ErrOIDCDisabled = errors.New("oidc provider disabled")
)

const (
    // Shortest path: increase access token TTL so Terraform runs won't expire mid-apply.
    // Future work will scope long-lived run tokens separately.
    defaultAccessTokenTTL  = 120 * time.Minute
    defaultRefreshTokenTTL = 24 * time.Hour
    defaultIDTokenTTL      = 15 * time.Minute
)

// ProviderDependencies holds the repositories required by the OIDC storage adapter.
type ProviderDependencies struct {
	Users           repository.UserRepository
	ServiceAccounts repository.ServiceAccountRepository
	Sessions        repository.SessionRepository
}

// Provider exposes the server-side OIDC endpoints wired through zitadel/oidc.
type Provider struct {
	Router  chi.Router
	Storage op.Storage
}

// loadOrGenerateSigningKey loads an RSA private key and its ID from disk, or generates and saves them if they don't exist.
// Returns the private key and the key ID (kid).
func loadOrGenerateSigningKey(keyPath string) (*rsa.PrivateKey, string, error) {
	// If keyPath is empty, we must generate a new key every time (not ideal but acceptable for dev)
	if keyPath == "" {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		return key, uuid.NewString(), err
	}

	keyIDPath := keyPath + ".kid"

	// Try to load existing key from disk
	keyData, err := os.ReadFile(keyPath)
	if err == nil {
		// Parse PEM-encoded private key
		block, _ := pem.Decode(keyData)
		if block == nil || block.Type != "RSA PRIVATE KEY" {
			return nil, "", fmt.Errorf("invalid PEM block in signing key")
		}
		privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, "", fmt.Errorf("parse signing key: %w", err)
		}

		// Load the key ID
		keyIDData, err := os.ReadFile(keyIDPath)
		if err != nil {
			return nil, "", fmt.Errorf("read key ID file: %w", err)
		}
		keyID := strings.TrimSpace(string(keyIDData))
		if keyID == "" {
			return nil, "", fmt.Errorf("key ID file is empty")
		}

		return privateKey, keyID, nil
	}

	// Generate new key if file doesn't exist
	if !os.IsNotExist(err) {
		return nil, "", fmt.Errorf("read signing key file: %w", err)
	}

	// Generate new 2048-bit RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", fmt.Errorf("generate signing key: %w", err)
	}

	// Generate a stable key ID
	keyID := uuid.NewString()

	// Save key to disk
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	})

	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return nil, "", fmt.Errorf("save signing key to disk: %w", err)
	}

	// Save key ID to disk
	if err := os.WriteFile(keyIDPath, []byte(keyID), 0600); err != nil {
		return nil, "", fmt.Errorf("save key ID to disk: %w", err)
	}

	return privateKey, keyID, nil
}

// NewOIDCProvider builds an OpenID provider instance when OIDC is enabled.
func NewOIDCProvider(ctx context.Context, cfg config.OIDCConfig, deps ProviderDependencies) (*Provider, error) {
	if cfg.Issuer == "" {
		return nil, ErrOIDCDisabled
	}
	if deps.Users == nil || deps.ServiceAccounts == nil || deps.Sessions == nil {
		return nil, fmt.Errorf("oidc storage dependencies incomplete")
	}

	storage, err := newProviderStorage(deps, cfg.SigningKeyPath)
	if err != nil {
		return nil, fmt.Errorf("initialise oidc storage: %w", err)
	}

	opConfig := &op.Config{
		CodeMethodS256:           true,
		AuthMethodPost:           true,
		GrantTypeRefreshToken:    true,
		SupportedScopes:          []string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail, oidc.ScopeOfflineAccess},
		SupportedClaims:          op.DefaultSupportedClaims,
		DefaultLogoutRedirectURI: "",
	}

	provider, err := op.NewProvider(opConfig, storage, op.StaticIssuer(cfg.Issuer), op.WithAllowInsecure())
	if err != nil {
		return nil, err
	}

	// Set the signer and issuer on the storage instance after the provider is created.
	storage.setSigner(provider.Crypto())
	storage.setIssuer(cfg.Issuer)

	return &Provider{
		Router:  op.CreateRouter(provider),
		Storage: storage,
	}, nil
}

// Handler exposes the chi.Router handling the OIDC endpoints.
func (p *Provider) Handler() chi.Router {
	return p.Router
}

type providerStorage struct {
	users           repository.UserRepository
	serviceAccounts repository.ServiceAccountRepository
	sessions        repository.SessionRepository

	mu            sync.Mutex
	authRequests  map[string]*authRequest
	authCodes     map[string]string
	refreshTokens map[string]*refreshToken
	deviceCodes   map[string]deviceAuthorizationEntry
	userCodes     map[string]string

	signingKey *rsaSigningKey
	issuer     string
	signer     op.Crypto
}

func (s *providerStorage) setSigner(signer op.Crypto) {
	s.signer = signer
}

func (s *providerStorage) setIssuer(issuer string) {
	s.issuer = issuer
}

func newProviderStorage(deps ProviderDependencies, keyPath string) (*providerStorage, error) {
	// Load persistent RSA key and key ID or generate and save new ones
	privateKey, keyID, err := loadOrGenerateSigningKey(keyPath)
	if err != nil {
		return nil, fmt.Errorf("load or generate signing key: %w", err)
	}

	return &providerStorage{
		users:           deps.Users,
		serviceAccounts: deps.ServiceAccounts,
		sessions:        deps.Sessions,
		authRequests:    make(map[string]*authRequest),
		authCodes:       make(map[string]string),
		refreshTokens:   make(map[string]*refreshToken),
		deviceCodes:     make(map[string]deviceAuthorizationEntry),
		userCodes:       make(map[string]string),
		signingKey: &rsaSigningKey{
			id:        keyID, // Use the persisted/loaded key ID
			algorithm: jose.RS256,
			key:       privateKey,
		},
	}, nil
}

func (s *providerStorage) Health(context.Context) error {
	return nil
}

func (s *providerStorage) CreateAuthRequest(ctx context.Context, req *oidc.AuthRequest, userID string) (op.AuthRequest, error) {
	if len(req.Prompt) == 1 && req.Prompt[0] == string(oidc.PromptNone) {
		return nil, oidc.ErrLoginRequired()
	}

	authReq := authRequestFromOIDC(req, userID)
	authReq.id = uuid.NewString()

	s.mu.Lock()
	s.authRequests[authReq.id] = authReq
	s.mu.Unlock()

	return authReq, nil
}

func (s *providerStorage) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.authRequests[id]
	if !ok {
		return nil, fmt.Errorf("auth request %s not found", id)
	}
	return req, nil
}

func (s *providerStorage) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.authCodes[code]
	if !ok {
		return nil, fmt.Errorf("authorization code invalid")
	}
	req, ok := s.authRequests[id]
	if !ok {
		return nil, fmt.Errorf("auth request %s not found", id)
	}
	return req, nil
}

func (s *providerStorage) SaveAuthCode(ctx context.Context, id string, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authCodes[code] = id
	return nil
}

func (s *providerStorage) DeleteAuthRequest(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.authRequests, id)
	for code, requestID := range s.authCodes {
		if requestID == id {
			delete(s.authCodes, code)
		}
	}
	return nil
}

func (s *providerStorage) createJWT(request op.TokenRequest, exp time.Time) (string, string, error) {
	jti := uuid.NewString()
	now := time.Now()

	// For access tokens, audience should be the API (gridapi), not the client
	// The client_id is included in a separate claim
	audience := []string{"gridapi"}

	claims := &oidc.IDTokenClaims{
		TokenClaims: oidc.TokenClaims{
			Issuer:     s.issuer,
			Subject:    request.GetSubject(),
			Audience:   oidc.Audience(audience),
			Expiration: oidc.FromTime(exp),
			IssuedAt:   oidc.FromTime(now),
			JWTID:      jti,
		},
		Claims: make(map[string]any),
	}

	key := &jose.JSONWebKey{Key: s.signingKey.key, Algorithm: string(s.signingKey.algorithm), KeyID: s.signingKey.id}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: s.signingKey.algorithm, Key: key}, nil)
	if err != nil {
		return "", "", err
	}
	token, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		return "", "", err
	}
	return token, jti, nil
}

func (s *providerStorage) CreateAccessToken(ctx context.Context, request op.TokenRequest) (string, time.Time, error) {
	exp := time.Now().Add(defaultAccessTokenTTL)
	token, jti, err := s.createJWT(request, exp)
	if err != nil {
		return "", time.Time{}, err
	}

	if err := s.persistSession(ctx, request, jti, "", exp); err != nil {
		return "", time.Time{}, err
	}

	return token, exp, nil
}

func (s *providerStorage) CreateAccessAndRefreshTokens(ctx context.Context, request op.TokenRequest, currentRefreshToken string) (string, string, time.Time, error) {
	if currentRefreshToken != "" {
		s.mu.Lock()
		if rt, ok := s.refreshTokens[currentRefreshToken]; ok {
			delete(s.refreshTokens, currentRefreshToken)
			go s.revokeSessionByJTI(context.Background(), rt.AccessToken) // rt.AccessToken holds the JTI
		}
		s.mu.Unlock()
	}

	exp := time.Now().Add(defaultAccessTokenTTL)
	accessToken, jti, err := s.createJWT(request, exp)
	if err != nil {
		return "", "", time.Time{}, err
	}

	refreshID := uuid.NewString()
	rt := &refreshToken{
		ID:            refreshID,
		Token:         refreshID,
		AuthTime:      time.Now(),
		AMR:           getAMR(request),
		Audience:      request.GetAudience(),
		UserID:        request.GetSubject(),
		ApplicationID: getClientID(request),
		Expiration:    time.Now().Add(defaultRefreshTokenTTL),
		Scopes:        request.GetScopes(),
		AccessToken:   jti, // Store JTI instead of opaque token
	}

	s.mu.Lock()
	s.refreshTokens[rt.ID] = rt
	s.mu.Unlock()

	if err := s.persistSession(ctx, request, jti, rt.Token, exp); err != nil {
		s.mu.Lock()
		delete(s.refreshTokens, rt.ID)
		s.mu.Unlock()
		return "", "", time.Time{}, err
	}

	return accessToken, rt.Token, exp, nil
}

func (s *providerStorage) TokenRequestByRefreshToken(ctx context.Context, token string) (op.RefreshTokenRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rt, ok := s.refreshTokens[token]
	if !ok {
		return nil, op.ErrInvalidRefreshToken
	}
	return refreshTokenRequestFromRefreshToken(rt), nil
}

func (s *providerStorage) TerminateSession(ctx context.Context, userID string, clientID string) error {
	// TODO: Implement full session termination in the database for the user/client.
	return nil
}

func (s *providerStorage) RevokeToken(ctx context.Context, tokenOrID string, userID string, clientID string) *oidc.Error {
	s.mu.Lock()
	if rt, ok := s.refreshTokens[tokenOrID]; ok {
		if rt.ApplicationID != clientID {
			s.mu.Unlock()
			return oidc.ErrInvalidClient().WithDescription("token belongs to another client")
		}
		delete(s.refreshTokens, tokenOrID)
		go s.revokeSessionByJTI(context.Background(), rt.AccessToken)
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	if err := s.revokeSessionByJTI(ctx, tokenOrID); err != nil {
		// Log the error, but don't necessarily return an OIDC error unless the spec requires it.
	}

	return nil
}

func (s *providerStorage) GetRefreshTokenInfo(ctx context.Context, clientID string, token string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rt, ok := s.refreshTokens[token]
	if !ok {
		return "", "", op.ErrInvalidRefreshToken
	}
	if rt.ApplicationID != clientID {
		return "", "", fmt.Errorf("token belongs to another client")
	}
	return rt.UserID, rt.ID, nil
}

func (s *providerStorage) SigningKey(ctx context.Context) (op.SigningKey, error) {
	return s.signingKey, nil
}

func (s *providerStorage) SignatureAlgorithms(context.Context) ([]jose.SignatureAlgorithm, error) {
	return []jose.SignatureAlgorithm{s.signingKey.algorithm}, nil
}

func (s *providerStorage) KeySet(ctx context.Context) ([]op.Key, error) {
	return []op.Key{&rsaPublicKey{signingKey: s.signingKey}}, nil
}

func (s *providerStorage) GetClientByClientID(ctx context.Context, clientID string) (op.Client, error) {
	sa, err := s.serviceAccounts.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return newServiceAccountClient(sa), nil
}

func (s *providerStorage) AuthorizeClientIDSecret(ctx context.Context, clientID, clientSecret string) error {
	sa, err := s.serviceAccounts.GetByClientID(ctx, clientID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(sa.ClientSecretHash), []byte(clientSecret)); err != nil {
		return fmt.Errorf("invalid client secret")
	}
	return nil
}

func (s *providerStorage) SetUserinfoFromScopes(context.Context, *oidc.UserInfo, string, string, []string) error {
	return nil
}

func (s *providerStorage) SetUserinfoFromRequest(ctx context.Context, userInfo *oidc.UserInfo, token op.IDTokenRequest, scopes []string) error {
	return s.populateUserInfo(ctx, userInfo, token.GetSubject(), token.GetClientID(), scopes)
}

func (s *providerStorage) SetUserinfoFromToken(ctx context.Context, userInfo *oidc.UserInfo, tokenID, subject, clientID string) error {
	session, err := s.sessions.GetByTokenHash(ctx, HashBearerToken(tokenID))
	if err != nil {
		return err
	}
	if session.Revoked {
		return errors.New("session is revoked")
	}
	return s.populateUserInfo(ctx, userInfo, subject, clientID, nil) // Scopes not available here
}

func (s *providerStorage) SetIntrospectionFromToken(ctx context.Context, resp *oidc.IntrospectionResponse, tokenID, subject, clientID string) error {
	session, err := s.sessions.GetByTokenHash(ctx, HashBearerToken(tokenID))
	if err != nil {
		if isNotFoundError(err) {
			resp.Active = false
			return nil
		}
		return err
	}
	if session.Revoked {
		resp.Active = false
		return nil
	}

	resp.Active = time.Now().Before(session.ExpiresAt)
	resp.Subject = subject
	resp.ClientID = clientID
	resp.JWTID = tokenID
	resp.Expiration = oidc.FromTime(session.ExpiresAt)
	resp.IssuedAt = oidc.FromTime(session.CreatedAt)

	return nil
}

func (s *providerStorage) GetPrivateClaimsFromScopes(context.Context, string, string, []string) (map[string]any, error) {
	return nil, nil
}

func (s *providerStorage) GetKeyByIDAndClientID(context.Context, string, string) (*jose.JSONWebKey, error) {
	return nil, fmt.Errorf("client keys not supported")
}

func (s *providerStorage) ValidateJWTProfileScopes(context.Context, string, []string) ([]string, error) {
	return []string{oidc.ScopeOpenID}, nil
}

func (s *providerStorage) StoreDeviceAuthorization(ctx context.Context, clientID, deviceCode, userCode string, expires time.Time, scopes []string) error {
	if _, err := s.serviceAccounts.GetByClientID(ctx, clientID); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.userCodes[userCode]; exists {
		return op.ErrDuplicateUserCode
	}

	entry := deviceAuthorizationEntry{
		deviceCode: deviceCode,
		userCode:   userCode,
		state: &op.DeviceAuthorizationState{
			ClientID: clientID,
			Scopes:   scopes,
			Expires:  expires,
		},
	}
	s.deviceCodes[deviceCode] = entry
	s.userCodes[userCode] = deviceCode
	return nil
}

func (s *providerStorage) GetDeviceAuthorizatonState(ctx context.Context, clientID, deviceCode string) (*op.DeviceAuthorizationState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.deviceCodes[deviceCode]
	if !ok || entry.state.ClientID != clientID {
		return nil, fmt.Errorf("device code not found")
	}
	return entry.state, nil
}

func (s *providerStorage) ClientCredentials(ctx context.Context, clientID, clientSecret string) (op.Client, error) {
	if err := s.AuthorizeClientIDSecret(ctx, clientID, clientSecret); err != nil {
		return nil, err
	}
	return &serviceAccountClient{
		id: clientID,
	}, nil
}

func (s *providerStorage) ClientCredentialsTokenRequest(ctx context.Context, clientID string, scopes []string) (op.TokenRequest, error) {
	sa, err := s.serviceAccounts.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("service account not found: %w", err)
	}

	if sa.Disabled {
		return nil, fmt.Errorf("service account is disabled")
	}

	// TODO: This only works for Service Accounts at the moment
	return &clientCredentialsTokenRequest{
		clientID: clientID,
		scopes:   scopes,
		subject:  ServiceAccountID(clientID),
	}, nil
}

type clientCredentialsTokenRequest struct {
	clientID string
	scopes   []string
	subject  string
}

func (r *clientCredentialsTokenRequest) GetSubject() string {
	return r.subject
}

func (r *clientCredentialsTokenRequest) GetAudience() []string {
	return []string{"gridapi"}
}

func (r *clientCredentialsTokenRequest) GetScopes() []string {
	return r.scopes
}

// --- helpers and internal types ---

type authRequest struct {
	*oidc.AuthRequest
	id           string
	userID       string
	createdAt    time.Time
	authTime     time.Time
	sessionState string
	done         bool
}

func authRequestFromOIDC(req *oidc.AuthRequest, userID string) *authRequest {
	return &authRequest{
		AuthRequest: req,
		userID:      userID,
		createdAt:   time.Now(),
		authTime:    time.Now(),
	}
}

func (a *authRequest) GetID() string {
	return a.id
}

func (a *authRequest) GetACR() string {
	return ""
}

func (a *authRequest) GetAMR() []string {
	if a.done {
		return []string{"pwd"}
	}
	return nil
}

func (a *authRequest) GetAudience() []string {
	return []string{a.ClientID}
}

func (a *authRequest) GetAuthTime() time.Time {
	return a.authTime
}

func (a *authRequest) GetClientID() string {
	return a.ClientID
}

func (a *authRequest) GetCodeChallenge() *oidc.CodeChallenge {
	if a.CodeChallenge == "" {
		return nil
	}
	return &oidc.CodeChallenge{
		Challenge: a.CodeChallenge,
		Method:    a.CodeChallengeMethod,
	}
}

func (a *authRequest) GetSubject() string {
	return a.userID
}

func (a *authRequest) Done() bool {
	return a.done
}

func (a *authRequest) GetNonce() string {
	return a.Nonce
}

func (a *authRequest) GetScopes() []string {
	return append([]string{}, a.AuthRequest.Scopes...)
}

func (a *authRequest) GetResponseType() oidc.ResponseType {
	return a.AuthRequest.ResponseType
}

func (a *authRequest) GetResponseMode() oidc.ResponseMode {
	return a.AuthRequest.ResponseMode
}

func (a *authRequest) GetState() string {
	return a.AuthRequest.State
}

func (a *authRequest) GetRedirectURI() string {
	return a.AuthRequest.RedirectURI
}

type refreshToken struct {
	ID            string
	Token         string
	AuthTime      time.Time
	AMR           []string
	Audience      []string
	UserID        string
	ApplicationID string
	Expiration    time.Time
	Scopes        []string
	AccessToken   string
}

type deviceAuthorizationEntry struct {
	deviceCode string
	userCode   string
	state      *op.DeviceAuthorizationState
}

func (s *providerStorage) populateUserInfo(ctx context.Context, info *oidc.UserInfo, userID, clientID string, scopes []string) error {
	user, err := s.users.GetBySubject(ctx, userID)
	if err != nil {
		return err
	}

	for _, scope := range scopes {
		switch scope {
		case oidc.ScopeOpenID:
			info.Subject = user.ID
		case oidc.ScopeEmail:
			info.Email = user.Email
		case oidc.ScopeProfile:
			info.PreferredUsername = user.Email
			info.Name = user.Name
		}
	}
	return nil
}

type rsaSigningKey struct {
	id        string
	algorithm jose.SignatureAlgorithm
	key       *rsa.PrivateKey
}

func (k *rsaSigningKey) SignatureAlgorithm() jose.SignatureAlgorithm {
	return k.algorithm
}

func (k *rsaSigningKey) Key() any {
	return k.key
}

func (k *rsaSigningKey) ID() string {
	return k.id
}

type rsaPublicKey struct {
	signingKey *rsaSigningKey
}

func (k *rsaPublicKey) ID() string {
	return k.signingKey.id
}

func (k *rsaPublicKey) Algorithm() jose.SignatureAlgorithm {
	return k.signingKey.algorithm
}

func (k *rsaPublicKey) Use() string {
	return "sig"
}

func (k *rsaPublicKey) Key() any {
	return &k.signingKey.key.PublicKey
}

type serviceAccountClient struct {
	id string
}

func newServiceAccountClient(sa *models.ServiceAccount) op.Client {
	return &serviceAccountClient{id: sa.ClientID}
}

func (c *serviceAccountClient) GetID() string {
	return c.id
}

func (c *serviceAccountClient) RedirectURIs() []string {
	return nil
}

func (c *serviceAccountClient) PostLogoutRedirectURIs() []string {
	return nil
}

func (c *serviceAccountClient) ApplicationType() op.ApplicationType {
	return op.ApplicationTypeWeb
}

func (c *serviceAccountClient) AuthMethod() oidc.AuthMethod {
	return oidc.AuthMethodPost
}

func (c *serviceAccountClient) ResponseTypes() []oidc.ResponseType {
	return []oidc.ResponseType{oidc.ResponseTypeCode}
}

func (c *serviceAccountClient) GrantTypes() []oidc.GrantType {
	return []oidc.GrantType{
		oidc.GrantTypeCode,
		oidc.GrantTypeRefreshToken,
		oidc.GrantTypeClientCredentials,
	}
}

func (c *serviceAccountClient) LoginURL(requestID string) string {
	return "/auth/login?id=" + requestID
}

func (c *serviceAccountClient) AccessTokenType() op.AccessTokenType {
	return op.AccessTokenTypeJWT
}

func (c *serviceAccountClient) IDTokenLifetime() time.Duration {
	return defaultIDTokenTTL
}

func (c *serviceAccountClient) DevMode() bool {
	return false
}

func (c *serviceAccountClient) RestrictAdditionalIdTokenScopes() func(scopes []string) []string {
	return func(scopes []string) []string { return scopes }
}

func (c *serviceAccountClient) RestrictAdditionalAccessTokenScopes() func(scopes []string) []string {
	return func(scopes []string) []string { return scopes }
}

func (c *serviceAccountClient) IsScopeAllowed(string) bool {
	return true
}

func (c *serviceAccountClient) IDTokenUserinfoClaimsAssertion() bool {
	return false
}

func (c *serviceAccountClient) ClockSkew() time.Duration {
	return 0
}

type refreshTokenRequest struct {
	token  *refreshToken
	scopes []string
}

func refreshTokenRequestFromRefreshToken(token *refreshToken) *refreshTokenRequest {
	return &refreshTokenRequest{
		token:  token,
		scopes: append([]string{}, token.Scopes...),
	}
}

func (r *refreshTokenRequest) GetScopes() []string {
	return r.scopes
}

func (r *refreshTokenRequest) GetAudience() []string {
	return r.token.Audience
}

func (r *refreshTokenRequest) GetSubject() string {
	return r.token.UserID
}

func (r *refreshTokenRequest) GetAMR() []string {
	return r.token.AMR
}

func (r *refreshTokenRequest) GetAuthTime() time.Time {
	return r.token.AuthTime
}

func (r *refreshTokenRequest) GetClientID() string {
	return r.token.ApplicationID
}

func (r *refreshTokenRequest) SetCurrentScopes(scopes []string) {
	if scopes == nil {
		r.scopes = append([]string{}, r.token.Scopes...)
		return
	}
	r.scopes = append([]string{}, scopes...)
}

func getAMR(request op.TokenRequest) []string {
	if authReq, ok := request.(op.AuthRequest); ok {
		return authReq.GetAMR()
	}
	return nil
}

func getClientID(request op.TokenRequest) string {
	if authReq, ok := request.(op.AuthRequest); ok {
		return authReq.GetClientID()
	}
	if ccReq, ok := request.(*clientCredentialsTokenRequest); ok {
		return ccReq.clientID
	}
	if rtReq, ok := request.(*refreshTokenRequest); ok {
		return rtReq.token.ApplicationID
	}
	return ""
}

func (s *providerStorage) persistSession(ctx context.Context, request op.TokenRequest, accessToken string, refreshToken string, expiresAt time.Time) error {
	if s.sessions == nil {
		return fmt.Errorf("session repository unavailable")
	}

	tokenHash := HashBearerToken(accessToken)
	now := time.Now()

	session := &models.Session{
		TokenHash:    tokenHash,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		CreatedAt:    now,
		LastUsedAt:   now,
	}

	subject := strings.TrimSpace(request.GetSubject())

	if subject != "" && s.users != nil {
		if user, err := s.users.GetBySubject(ctx, subject); err == nil {
			userID := user.ID
			session.UserID = &userID
		} else if !isNotFoundError(err) {
			return fmt.Errorf("lookup user %s: %w", subject, err)
		}
	}

	if session.UserID == nil && s.serviceAccounts != nil {
		var clientID string
		if cr, ok := request.(*clientCredentialsTokenRequest); ok {
			clientID = cr.clientID
		}
		if clientID == "" {
			if authReq, ok := request.(op.AuthRequest); ok {
				clientID = strings.TrimSpace(authReq.GetClientID())
			}
		}

		if clientID != "" {
			if sa, err := s.serviceAccounts.GetByClientID(ctx, clientID); err == nil {
				serviceAccountID := sa.ID
				session.ServiceAccountID = &serviceAccountID
			} else if !isNotFoundError(err) {
				return fmt.Errorf("lookup service account %s: %w", clientID, err)
			}
		}
	}

	if session.UserID == nil && session.ServiceAccountID == nil {
		return fmt.Errorf("unknown token subject %q", subject)
	}

	if err := s.sessions.Create(ctx, session); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *providerStorage) revokeSessionByJTI(ctx context.Context, jti string) error {
	if s.sessions == nil {
		return nil
	}
	hash := HashBearerToken(jti)
	session, err := s.sessions.GetByTokenHash(ctx, hash)
	if err != nil {
		if isNotFoundError(err) {
			return nil
		}
		return err
	}
	return s.sessions.Revoke(ctx, session.ID)
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found")
}
