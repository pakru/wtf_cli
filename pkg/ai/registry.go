package ai

import (
	"fmt"
	"log/slog"
	"sync"

	"wtf_cli/pkg/ai/auth"
	"wtf_cli/pkg/config"
)

// ProviderType represents a supported LLM provider.
type ProviderType string

const (
	ProviderOpenRouter ProviderType = "openrouter"
	ProviderOpenAI     ProviderType = "openai"
	ProviderCopilot    ProviderType = "copilot"
	ProviderAnthropic  ProviderType = "anthropic"
	ProviderGoogle     ProviderType = "google"
)

// ProviderConfig holds configuration for creating a provider.
type ProviderConfig struct {
	Type        ProviderType
	Config      config.Config
	AuthManager *auth.AuthManager
}

// ProviderFactory is a function that creates a Provider from config.
type ProviderFactory func(cfg ProviderConfig) (Provider, error)

// ProviderInfo describes a registered provider.
type ProviderInfo struct {
	Type        ProviderType
	Name        string
	Description string
	AuthMethod  string // "api_key", "oauth_device", "oauth_pkce", "none"
	RequiresKey bool
}

// Registry manages provider factories and instantiation.
type Registry struct {
	mu        sync.RWMutex
	factories map[ProviderType]ProviderFactory
	info      map[ProviderType]ProviderInfo
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[ProviderType]ProviderFactory),
		info:      make(map[ProviderType]ProviderInfo),
	}
}

// Register adds a provider factory to the registry.
func (r *Registry) Register(info ProviderInfo, factory ProviderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[info.Type] = factory
	r.info[info.Type] = info
	slog.Debug("provider_register",
		"type", info.Type,
		"name", info.Name,
		"auth_method", info.AuthMethod,
		"requires_key", info.RequiresKey,
	)
}

// GetProvider creates a provider instance by type.
func (r *Registry) GetProvider(cfg ProviderConfig) (Provider, error) {
	r.mu.RLock()
	factory, ok := r.factories[cfg.Type]
	r.mu.RUnlock()

	if !ok {
		slog.Debug("provider_get_unknown", "type", cfg.Type)
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}

	slog.Debug("provider_get", "type", cfg.Type)
	return factory(cfg)
}

// ListProviders returns information about all registered providers.
func (r *Registry) ListProviders() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]ProviderInfo, 0, len(r.info))
	for _, info := range r.info {
		providers = append(providers, info)
	}
	return providers
}

// GetProviderInfo returns information about a specific provider.
func (r *Registry) GetProviderInfo(providerType ProviderType) (ProviderInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.info[providerType]
	return info, ok
}

// IsRegistered checks if a provider type is registered.
func (r *Registry) IsRegistered(providerType ProviderType) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.factories[providerType]
	return ok
}

// DefaultRegistry is the global provider registry.
var DefaultRegistry = NewRegistry()

// RegisterProvider registers a provider with the default registry.
func RegisterProvider(info ProviderInfo, factory ProviderFactory) {
	DefaultRegistry.Register(info, factory)
}

// GetProvider creates a provider from the default registry.
func GetProvider(cfg ProviderConfig) (Provider, error) {
	return DefaultRegistry.GetProvider(cfg)
}

// ListProviders returns all providers from the default registry.
func ListProviders() []ProviderInfo {
	return DefaultRegistry.ListProviders()
}

// SupportedProviders returns a list of all supported provider types.
func SupportedProviders() []ProviderType {
	return []ProviderType{
		ProviderOpenRouter,
		ProviderOpenAI,
		ProviderCopilot,
		ProviderAnthropic,
		ProviderGoogle,
	}
}

// ValidateProviderType checks if a provider type string is valid.
func ValidateProviderType(s string) (ProviderType, bool) {
	pt := ProviderType(s)
	for _, supported := range SupportedProviders() {
		if pt == supported {
			return pt, true
		}
	}
	return "", false
}

// GetProviderFromConfig creates a provider based on the config's LLMProvider setting.
// It handles auth manager creation and provider instantiation.
func GetProviderFromConfig(cfg config.Config) (Provider, error) {
	providerType, ok := ValidateProviderType(cfg.LLMProvider)
	if !ok {
		slog.Debug("provider_invalid_fallback", "requested", cfg.LLMProvider, "fallback", ProviderOpenRouter)
		providerType = ProviderOpenRouter
	}

	var authMgr *auth.AuthManager
	if providerType == ProviderOpenAI {
		authMgr = auth.NewAuthManager(auth.DefaultAuthPath())
	}

	slog.Debug("provider_from_config",
		"provider", providerType,
		"auth_manager", authMgr != nil,
	)
	providerCfg := ProviderConfig{
		Type:        providerType,
		Config:      cfg,
		AuthManager: authMgr,
	}

	return GetProvider(providerCfg)
}
