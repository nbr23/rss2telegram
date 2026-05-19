package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type FeedConfig struct {
	Title string `yaml:"title"`
	URL   string `yaml:"url"`
}

type Config struct {
	Interval           time.Duration `yaml:"interval"`
	StateFile          string        `yaml:"state_file"`
	MaxSeenPerFeed     int           `yaml:"max_seen_per_feed"`
	DisableLinkPreview bool          `yaml:"disable_link_preview"`
	Feeds              []FeedConfig  `yaml:"feeds"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var raw struct {
		Interval           string       `yaml:"interval"`
		StateFile          string       `yaml:"state_file"`
		MaxSeenPerFeed     int          `yaml:"max_seen_per_feed"`
		DisableLinkPreview bool         `yaml:"disable_link_preview"`
		Feeds              []FeedConfig `yaml:"feeds"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg := &Config{
		StateFile:          raw.StateFile,
		MaxSeenPerFeed:     raw.MaxSeenPerFeed,
		DisableLinkPreview: raw.DisableLinkPreview,
		Feeds:              raw.Feeds,
	}

	if raw.Interval == "" {
		return nil, fmt.Errorf("interval is required")
	}
	d, err := time.ParseDuration(raw.Interval)
	if err != nil {
		return nil, fmt.Errorf("invalid interval %q: %w", raw.Interval, err)
	}
	if d <= 0 {
		return nil, fmt.Errorf("interval must be > 0")
	}
	cfg.Interval = d

	if cfg.StateFile == "" {
		cfg.StateFile = "./state.json"
	}
	if cfg.MaxSeenPerFeed <= 0 {
		cfg.MaxSeenPerFeed = 500
	}

	if len(cfg.Feeds) == 0 {
		return nil, fmt.Errorf("at least one feed is required")
	}

	seen := make(map[string]bool, len(cfg.Feeds))
	for i, f := range cfg.Feeds {
		if f.Title == "" {
			return nil, fmt.Errorf("feed[%d]: title is required", i)
		}
		if f.URL == "" {
			return nil, fmt.Errorf("feed[%d] %q: url is required", i, f.Title)
		}
		if seen[f.URL] {
			return nil, fmt.Errorf("duplicate feed url: %s", f.URL)
		}
		seen[f.URL] = true
	}

	return cfg, nil
}
