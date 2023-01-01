package session_validator

import (
	"errors"

	"github.com/gofrs/uuid"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/config"
	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/zap"
)

type Plugin struct {
	config   PluginConfig
	logger   *zap.Logger
	eventBus event.Bus

	eventID string

	validators map[string]validator
}

func (p Plugin) Name() string {
	return "Session Validator"
}

func (p Plugin) Version() string {
	return "internal"
}

func (p Plugin) Init() {}

func (p *Plugin) Load(cfg map[string]any) error {
	if err := config.Unmarshal(cfg, &p.config); err != nil {
		return err
	}

	if !p.config.SessionValidator.Enable {
		return infrared.ErrPluginViaConfigDisabled
	}
	return nil
}

func (p *Plugin) Reload(cfg map[string]any) error {
	if err := p.Load(cfg); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) Enable(api infrared.PluginAPI) error {
	p.logger = api.Logger()
	p.eventBus = api.EventBus()

	p.eventID = p.eventBus.HandleFunc(p.onPostProcessing, infrared.PostProcessingEventTopic)

	return nil
}

func (p Plugin) Disable() error {
	p.eventBus.DetachRecipient(p.eventID)
	return nil
}

func (p Plugin) onPostProcessing(e event.Event) (any, error) {
	switch data := e.Data.(type) {
	case infrared.PostConnProcessingEvent:
		player := data.Player
		validator := p.validators[player.GatewayID()]
		playerUUID, err := validator.Validate(player)
		if err != nil {
			p.logger.Error("failed to validate session",
				zap.Error(err),
			)
			return nil, err
		}

		p.logger.Debug("validated player",
			zap.String("uuid", playerUUID.String()),
		)
	}
	return nil, nil
}

type validator interface {
	Validate(infrared.Player) (uuid.UUID, error)
}

type cachedSessionService struct {
	storage             storage
	service             validator
	successDisconnector infrared.PlayerDisconnecter
	failureDisconnector infrared.PlayerDisconnecter
}

func (c cachedSessionService) Validate(player infrared.Player) (uuid.UUID, error) {
	playerUUID, err := c.storage.GetValidation(player.Username(), player.RemoteIP())
	if err != nil {
		if errors.Is(err, errValidationNotFound) {
			playerUUID, err = c.validate(player)
			if err != nil {
				c.failureDisconnector.DisconnectPlayer(player)
				return uuid.Nil, nil
			}
			c.successDisconnector.DisconnectPlayer(player)
			return playerUUID, nil
		}
	}

	return playerUUID, nil
}

func (c cachedSessionService) validate(player infrared.Player) (uuid.UUID, error) {
	playerUUID, err := c.service.Validate(player)
	if err != nil {
		return uuid.Nil, err
	}

	if err := c.storage.PutValidation(player.Username(), player.RemoteIP(), playerUUID); err != nil {
		return uuid.Nil, err
	}

	return playerUUID, nil
}

type sessionServerValidator struct {
	baseURL string
}

func (s sessionServerValidator) Validate(player infrared.Player) (uuid.UUID, error) {

}

func 
docker build -f build/package/Dockerfile --no-cache -t haveachin/infrared:latest https://github.com/haveachin/infrared.git#v2.0.0-alpha.11