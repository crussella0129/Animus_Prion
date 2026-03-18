package llm

import (
	"fmt"
	"log/slog"

	"github.com/crussella0129/Animus_Prion/internal/config"
)

// Shutdowner is implemented by providers that manage external processes.
type Shutdowner interface {
	Shutdown() error
}

// NewProvider creates an LLM provider from configuration.
//   - "native": auto-detects hardware, launches the best inference backend
//   - "local": connects to an already-running OpenAI-compatible endpoint
//   - "anthropic": Anthropic Messages API
func NewProvider(cfg *config.Config) (Provider, error) {
	switch cfg.Model.Provider {
	case "native":
		return newNativeAutoDetect(cfg)

	case "local", "openai-compatible":
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

// newNativeAutoDetect handles the "native" provider with hardware auto-detection.
// 1. Try the explicitly configured model first
// 2. If not found, detect hardware and recommend the best backend
// 3. If the recommended model exists locally, use it
// 4. If nothing is available, return a clear error with setup instructions
func newNativeAutoDetect(cfg *config.Config) (Provider, error) {
	// Step 1: Try explicitly configured model
	modelPath, err := FindModel(cfg.Model.ModelName)
	if err == nil {
		// Model found — detect whether to use llama.cpp or BitNet server
		hw := DetectHardware()
		rec := RecommendBackend(hw)

		serverPath := ""
		if rec.Backend == BackendBitNet {
			// Check if BitNet server is available
			bp, _ := FindBitNetServer()
			if bp != "" {
				serverPath = bp
				slog.Info("auto-detect: using BitNet server", "reason", rec.Reason)
			}
			// If BitNet server not found, fall back to llama.cpp server
		}

		if serverPath == "" {
			slog.Info("auto-detect: using llama.cpp server", "reason", rec.Reason)
		}

		return NewNativeProvider(NativeProviderConfig{
			ModelPath:     modelPath,
			GPULayers:     cfg.Model.GPULayers,
			ContextLength: cfg.Model.ContextLength,
			SizeTier:      cfg.Model.SizeTier,
			ServerPath:    serverPath, // empty = use FindLlamaServer default
		})
	}

	// Step 2: Configured model not found — auto-detect and try recommended model
	hw := DetectHardware()
	rec := RecommendBackend(hw)
	slog.Info("auto-detect", "backend", rec.Backend, "model", rec.ModelName, "reason", rec.Reason)

	modelPath, err = FindModel(rec.ModelName)
	if err == nil {
		serverPath := ""
		if rec.Backend == BackendBitNet {
			bp, _ := FindBitNetServer()
			if bp != "" {
				serverPath = bp
			}
		}

		return NewNativeProvider(NativeProviderConfig{
			ModelPath:     modelPath,
			GPULayers:     cfg.Model.GPULayers,
			ContextLength: cfg.Model.ContextLength,
			SizeTier:      cfg.Model.SizeTier,
			ServerPath:    serverPath,
		})
	}

	// Step 3: Fall back to base_url if configured
	if cfg.Model.BaseURL != "" {
		return NewLocalProvider(LocalProviderConfig{
			BaseURL:       cfg.Model.BaseURL,
			APIKey:        cfg.Model.APIKey,
			Model:         cfg.Model.ModelName,
			ContextLength: cfg.Model.ContextLength,
			SizeTier:      cfg.Model.SizeTier,
		}), nil
	}

	// Step 4: Nothing available
	return nil, fmt.Errorf(
		"no model found.\n"+
			"  Hardware: %s, %d CPU cores, %d MB RAM\n"+
			"  GPU: %s (%d MB VRAM)\n"+
			"  Recommended: %s (%s, %s)\n\n"+
			"  Run 'prion setup' to download the recommended model,\n"+
			"  or place a .gguf model in ~/.animus_prion/models/",
		hw.OS+"/"+hw.Arch, hw.CPUCores, hw.RAMMiB,
		hw.GPU.Name, hw.GPU.VRAMMiB,
		rec.ModelName, rec.ModelSize, rec.Backend,
	)
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
