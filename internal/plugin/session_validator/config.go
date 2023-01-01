package session_validator

import (
	"fmt"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock"
	"github.com/haveachin/infrared/internal/pkg/java"
	"github.com/imdario/mergo"
)

type sessionValidatorConfig struct {
	GatewayIDs              []string                `mapstructure:"gatewayIds"`
	CPSThreshold            int                     `mapstructure:"cpsThreshold"`
	SessionValidatedMessage string                  `mapstructure:"sessionValidateMessage"`
	SessionValidatedStatus  java.ServerStatusConfig `mapstructure:"sessionValidateStatus"`
	Redis                   redisConfig             `mapstructure:"redis"`
}

type PluginConfig struct {
	SessionValidator struct {
		Enable            bool                              `mapstructure:"enabled"`
		SessionValidators map[string]sessionValidatorConfig `mapstructure:"sessionValidator"`
	} `mapstructure:"sessionValidator"`
	Defaults struct {
		SessionValidator sessionValidatorConfig `mapstructure:"sessionValidator"`
	} `mapstructure:"defaults"`
}

func (cfg PluginConfig) loadSessionValidatorConfigs() (map[string]validator, error) {
	validators := map[string]validator{}
	for _, svCfg := range cfg.SessionValidator.SessionValidators {
		if err := mergo.Merge(&svCfg, cfg.Defaults.SessionValidator); err != nil {
			return nil, err
		}

		storage, err := newRedis(svCfg.Redis)
		if err != nil {
			return nil, err
		}

		statusJSON, err := java.NewServerStatus(svCfg.SessionValidatedStatus)
		if err != nil {
			return nil, err
		}

		javaSuccessDisconnecter, err := java.NewPlayerDisconnecter(statusJSON.ResponseJSON(), svCfg.SessionValidatedMessage)
		if err != nil {
			return nil, err
		}

		successPlayerDisconnecters := map[infrared.Edition]infrared.PlayerDisconnecter{
			infrared.JavaEdition:    javaSuccessDisconnecter,
			infrared.BedrockEdition: bedrock.NewPlayerDisconnecter(svCfg.SessionValidatedMessage),
		}

		for _, gID := range svCfg.GatewayIDs {
			_, ok := validators[gID]
			if ok {
				return nil, fmt.Errorf("server with ID %q already has a traffic limiter", gID)
			}

			validators[gID] = cachedSessionService{
				storage:             storage,
				successDisconnector: infrared.NewMultiPlayerDisconnecter(successPlayerDisconnecters),
			}
		}
	}

	return validators, nil
}
