package config

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	hl "github.com/Logarithm-Labs/go-hyperliquid/hyperliquid"
	"github.com/bitfield/script"
)

type HyperformanceConfig struct {
	Comments         string             `json:"comments"`
	SecretKey        string             `json:"secret_key"`
	CopyAddress      string             `json:"copy_address,omitempty"`
	PasteAddress     string             `json:"account_address"`
	CoinRiskMap      map[string]float64 `json:"coins"`
	DisableAloEngine bool               `json:"disable_alo_engine,omitempty"`
	DisableIocEngine bool               `json:"disable_ioc_engine,omitempty"`
}

func LoadConfigWithOverride(path string) (*HyperformanceConfig, error) {
	if path != "" {
		return ParseConfig(path)
	}
	return LoadConfig()
}

func LoadConfig() (*HyperformanceConfig, error) {
	f, err := script.FindFiles(".").
		MatchRegexp(regexp.MustCompile(`(^|/)config\.json$`)).
		First(1).
		String()
	if err != nil {
		return nil, fmt.Errorf("find config.json: %w", err)
	}
	f = strings.TrimSpace(f)
	if f == "" {
		f, err = script.FindFiles("..").
			MatchRegexp(regexp.MustCompile(`(^|/)config.*\.json$`)).
			First(1).
			String()
		if err != nil {
			return nil, fmt.Errorf("find config*.json: %w", err)
		}
		f = strings.TrimSpace(f)
	}
	if f == "" {
		return nil, fmt.Errorf("no config file found")
	}
	return ParseConfig(f)
}

func ParseConfig(p string) (*HyperformanceConfig, error) {
	d, err := script.File(p).String()
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c HyperformanceConfig
	if err := json.Unmarshal([]byte(d), &c); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &c, nil
}

func NewHyper() (*hl.Hyperliquid, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	h := hl.NewHyperliquid(&hl.HyperliquidClientConfig{
		IsMainnet:      true,
		AccountAddress: cfg.PasteAddress,
		PrivateKey:     cfg.SecretKey,
	})
	h.SetDebugActive()
	return h, nil
}

func BuildMetaMap(r *hl.Meta) (map[string]hl.AssetInfo, error) {
	m := make(map[string]hl.AssetInfo)
	for i, a := range r.Universe {
		m[a.Name] = hl.AssetInfo{SzDecimals: a.SzDecimals, AssetID: i, SpotName: a.Name}
	}
	return m, nil
}
