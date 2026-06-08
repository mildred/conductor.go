package config

import (
	"fmt"
	"os"
	"path"

	"github.com/BurntSushi/toml"

	"github.com/mildred/conductor.go/src/dirs"
)

var ConfigFile string = path.Join(dirs.ConfigHome, dirs.SelfName+".toml")

type Config struct {
	ULAPrefix string `toml:"ula_prefix"`
}

func InstallConfig() error {
	modified := false

	cfg, err := ReadConfig()
	if err != nil {
		return err
	}

	if cfg.ULAPrefix == "" {
		cfg.ULAPrefix = GenerateRandomULAPrefix(64)
		fmt.Printf(": ula_prefix = %q\n", cfg.ULAPrefix)
		modified = true
	}

	if modified {
		err = cfg.Write()
		if err != nil {
			return err
		}
	}

	return nil
}

func ReadConfig() (*Config, error) {
	cfg := &Config{}
	_, err := toml.DecodeFile(ConfigFile, cfg)
	if err != nil && os.IsNotExist(err) {
		err = nil
	}

	return cfg, err
}

func (cfg *Config) Write() error {
	f, err := os.Create(ConfigFile)
	if err != nil {
		return err
	}

	defer f.Close()

	return toml.NewEncoder(f).Encode(cfg)
}
