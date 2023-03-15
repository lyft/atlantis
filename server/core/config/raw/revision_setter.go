package raw

import (
	"os"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/runatlantis/atlantis/server/core/config/valid"
)

type PRRevision struct {
	Username    string `yaml:"username" json:"username"`
	PasswordEnv string `yaml:"password_env" json:"password_env"`
	URL         string `yaml:"url" json:"url"`
}

func (p PRRevision) Validate() error {
	return validation.ValidateStruct(&p,
		validation.Field(p.Username, validation.Required),
		validation.Field(p.PasswordEnv, validation.Required),
		validation.Field(p.URL, validation.Required),
	)
}

func (p PRRevision) ToValid() valid.PRRevision {
	return valid.PRRevision{
		Username: p.Username,
		Password: os.Getenv(p.PasswordEnv),
		URL:      p.URL,
	}
}
