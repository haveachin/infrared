package session_validator

import (
	"fmt"
	"time"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock"
	"github.com/haveachin/infrared/internal/pkg/java"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/status"
	"github.com/imdario/mergo"
)

type sessionValidatorConfig struct {
	ServerIDs                 []string      `mapstructure:"ServerIds"`
	CPSThreshold              int           `mapstructure:"cpsThreshold"`
	EncryptionResponseTimeout time.Duration `mapstructure:"encryptionResponseTimeout"`
	SessionServerBaseURL      string        `mapstructure:"sessionServerBaseURL"`
	ValidationSuccessMessage  string        `mapstructure:"validationSuccessMessage"`
	ValidationFailureMessage  string        `mapstructure:"validationFailureMessage"`
	Redis                     redisConfig   `mapstructure:"redis"`
}

type PluginConfig struct {
	SessionValidator struct {
		Enable            bool                              `mapstructure:"enable"`
		SessionValidators map[string]sessionValidatorConfig `mapstructure:"sessionValidators"`
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

		javaSuccessDisconnecter, err := java.NewPlayerDisconnecter(status.ResponseJSON{}, svCfg.ValidationSuccessMessage)
		if err != nil {
			return nil, err
		}

		successPlayerDisconnecters := map[infrared.Edition]infrared.PlayerDisconnecter{
			infrared.JavaEdition:    javaSuccessDisconnecter,
			infrared.BedrockEdition: bedrock.NewPlayerDisconnecter(svCfg.ValidationSuccessMessage),
		}

		javaFailureDisconnecter, err := java.NewPlayerDisconnecter(status.ResponseJSON{}, svCfg.ValidationFailureMessage)
		if err != nil {
			return nil, err
		}

		failurePlayerDisconnecters := map[infrared.Edition]infrared.PlayerDisconnecter{
			infrared.JavaEdition:    javaFailureDisconnecter,
			infrared.BedrockEdition: bedrock.NewPlayerDisconnecter(svCfg.ValidationFailureMessage),
		}

		for _, sID := range svCfg.ServerIDs {
			_, ok := validators[sID]
			if ok {
				return nil, fmt.Errorf("server with ID %q already has a traffic limiter", sID)
			}

			enc, pubKey, err := java.NewDefaultSessionEncrypter()
			if err != nil {
				return nil, err
			}

			validators[sID] = storageValidator{
				storage: storage,
				service: javaValidator{
					auth: &java.HTTPSessionAuthenticator{
						BaseURL: svCfg.SessionServerBaseURL,
					},
					enc:            enc,
					encRespTimeout: svCfg.EncryptionResponseTimeout,
				}.Validator("", pubKey),
				successDisconnector: infrared.NewMultiPlayerDisconnecter(successPlayerDisconnecters),
				failureDisconnector: infrared.NewMultiPlayerDisconnecter(failurePlayerDisconnecters),
			}
		}
	}

	return validators, nil
}
