package llm

import (
	"fmt"

	"github.com/crussella0129/Animus_Prion/internal/config"
)

// NewProvider creates an LLM provider from configuration.
func NewProvider(cfg *config.Config) (Provider, error) {
	switch cfg.Model.Provider {
	case "openai":
		return NewOpenAIProvider(OpenAIConfig{
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

	case "native":
		// Native provider (llama.cpp) will be implemented later
		return nil, fmt.Errorf("native provider not yet implemented — use openai or anthropic")

	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Model.Provider)
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
