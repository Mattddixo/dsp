package common

import (
	"github.com/Mattddixo/dsp/config"
	"github.com/urfave/cli/v2"
)

// GetConfig retrieves the config from the context
func GetConfig(c *cli.Context) (*config.Config, error) {
	return config.GetConfigFromContext(c.Context)
}
