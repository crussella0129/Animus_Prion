package llm

import (
	"fmt"

	"github.com/crussella0129/Animus_Prion/internal/config"
)

// Shutdowner is implemented by providers that manage external processes.
type Shutdowner interface {
	Shutdown() error
}

// NewProvider creates an LLM provider from configuration.
//   - "native": launches llama-server as a managed subprocess (recommended)
//   - "local": connects to an already-running OpenAI-compatible endpoint
//   - "anthropic": Anthropic Messages API
func NewProvider(cfg *config.Config) (Provider, error) {
	switch cfg.Model.Provider {
	case "native":
		// Managed subprocess: find model, launch llama-server, compose with LocalProvider
		modelPath, err := FindModel(cfg.Model.ModelName)
		if err != nil {
			// Fall back to model_path if model_name didn't resolve
			if cfg.Model.BaseURL != "" {
				// They have a base_url — treat as "local" instead
				return NewLocalProvider(LocalProviderConfig{
					BaseURL:       cfg.Model.BaseURL,
					APIKey:        cfg.Model.APIKey,
					Model:         cfg.Model.ModelName,
					ContextLength: cfg.Model.ContextLength,
					SizeTier:      cfg.Model.SizeTier,
				}), nil
			}
			return nil, err
		}
		return NewNativeProvider(NativeProviderConfig{
			ModelPath:     modelPath,
			GPULayers:     cfg.Model.GPULayers,
			ContextLength: cfg.Model.ContextLength,
			SizeTier:      cfg.Model.SizeTier,
		})

	case "local", "openai-compatible":
		// Existing server — user manages llama-server/vLLM/LM Studio themselves
		return NewLocalProvider(LocalProviderConfig{
			BaseURL:       cfg.Model.BaseURL,
			APIKey:        cfg.Model.APIKey,
			Model:         cfg.Model.ModelName,
			ContextLength: cfg.Model.ContextLength,
			SizeTier:      cfg.Model.SizeTier,
		}), nil

	case "anthropic":
		return NewAnthropicProvider(AnthropicConfig{
			APIKey:        cfg.Model.APIKey,
			Model:         cfg.Model.ModelName,
			ContextLength: cfg.Model.ContextLength,
		}), nil

	default:
		return nil, fmt.Errorf("unknown provider: %s (use 'native', 'local', or 'anthropic')", cfg.Model.Provider)
	}
}

// NewProviderWithFallback tries the preferred provider, then falls back.
func NewProviderWithFallback(cfg *config.Config, fallbacks ...string) (Provider, error) {
	p, err := NewProvider(cfg)
	if err == nil && p.Available() {
		return p, nil
	}

	for _, fb := range fallbacks {
		fbCfg := *cfg
		fbCfg.Model.Provider = fb
		p, err = NewProvider(&fbCfg)
		if err == nil && p.Available() {
			return p, nil
		}
	}

	return nil, fmt.Errorf("no available provider (tried: %s, %v)", cfg.Model.Provider, fallbacks)
}
