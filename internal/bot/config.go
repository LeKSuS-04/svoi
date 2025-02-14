package bot

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/sethvargo/go-envconfig"
	"gopkg.in/yaml.v3"
)

type StickerSetConfig struct {
	Name              string   `yaml:"name"`
	ExcludeStickerIDs []string `yaml:"exclude_sticker_ids"`
}

type Config struct {
	Debug       bool               `env:"DEBUG"`
	BotToken    string             `env:"BOT_TOKEN"`
	SqlitePath  string             `yaml:"sqlite_path" env:"SQLITE_PATH"`
	StickerSets []StickerSetConfig `yaml:"sticker_sets"`
	AdminIDs    []int64            `yaml:"admin_ids"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	config := &Config{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}

	if err := envconfig.Process(context.Background(), config); err != nil {
		return nil, fmt.Errorf("process envconfig: %w", err)
	}

	return config, nil
}

func (c *Config) IsAdmin(id int64) bool {
	return slices.Contains(c.AdminIDs, id)
}
