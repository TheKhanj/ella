package config

import (
	"os"
)

func ReadConfig(path string, cfg *Config) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return cfg.UnmarshalJSON(b)
}
