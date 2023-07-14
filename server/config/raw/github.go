package raw

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/runatlantis/atlantis/server/config/valid"
)

type Github struct {
	GatewayAppInstallationID  int64 `yaml:"gateway_app_installation_id" json:"gateway_app_installation_id"`
	TemporalAppInstallationID int64 `yaml:"temporal_app_installation_id" json:"temporal_app_installation_id"`
}

func (g *Github) Validate() error {
	return validation.ValidateStruct(g,
		validation.Field(&g.GatewayAppInstallationID, validation.Required),
		validation.Field(&g.TemporalAppInstallationID, validation.Required))
}

func (g *Github) ToValid() valid.Github {
	return valid.Github{
		GatewayAppInstallationID:  g.GatewayAppInstallationID,
		TemporalAppInstallationID: g.TemporalAppInstallationID,
	}
}
