package raw

import (
	"os"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/runatlantis/atlantis/server/core/config/valid"
)

type RevisionSetter struct {
	Enabled bool    `yaml:"enabled" json:"enabled"`
	Config  *Config `yaml:"config" json:"config"`
}

func (p RevisionSetter) Validate() error {
	return validation.ValidateStruct(&p,
		validation.Field(&p.Enabled, validation.Required),
		validation.Field(&p.Config),
	)
}

type Config struct {
	Username    string `yaml:"username" json:"username"`
	PasswordEnv string `yaml:"password_env" json:"password_env"`
	URL         string `yaml:"url" json:"url"`
}

func (c Config) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(c.Username, validation.Required),
		validation.Field(c.PasswordEnv, validation.Required),
		validation.Field(c.URL, validation.Required),
	)
}

func (c Config) ToValid() valid.RevisionSetterConfig {
	return valid.RevisionSetterConfig{
		Username: c.Username,
		Password: os.Getenv(c.PasswordEnv),
		URL:      c.URL,
	}
}

func (p RevisionSetter) ToValid() valid.RevisionSetter {
	var revSetterCfg valid.RevisionSetterConfig
	if p.Config != nil {
		revSetterCfg = p.Config.ToValid()
	}
	return valid.RevisionSetter{
		Enabled: p.Enabled,
		Config:  revSetterCfg,
	}
}
