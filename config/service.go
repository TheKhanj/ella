package config

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

func (this *Config) GetServices() ([]*Service, error) {
	ret := make([]*Service, 0)

	for _, s := range this.Services {
		if globStr, ok := s.(string); ok {
			err := this.globInclude(&ret, globStr)
			if err != nil {
				return nil, err
			}
		} else if mp, ok := s.(map[string]any); ok {
			b, err := json.Marshal(mp)
			if err != nil {
				return nil, err
			}
			var s Service
			err = s.UnmarshalJSON(b)
			if err != nil {
				return nil, err
			}
			ret = append(ret, &s)
		} else {
			return nil, fmt.Errorf("invalid service config: %v", s)
		}
	}

	return ret, nil
}

func (this *Config) globInclude(
	services *[]*Service, pattern string,
) error {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, file := range files {
		var cfg Config
		err = ReadConfig(file, &cfg)
		if err != nil {
			return err
		}
		subServices, err := cfg.GetServices()
		if err != nil {
			return err
		}
		*services = append(*services, subServices...)
	}

	return nil
}
