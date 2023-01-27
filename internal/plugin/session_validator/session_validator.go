package session_validator

import (
	"errors"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/config"
	"github.com/haveachin/infrared/internal/pkg/java"
	"github.com/haveachin/infrared/internal/pkg/java/protocol"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/login"
	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/zap"
)

type Plugin struct {
	config   PluginConfig
	logger   *zap.Logger
	eventBus event.Bus

	eventID string

	validators sync.Map
	cpsCounter cpsCounter
}

func (p *Plugin) Name() string {
	return "Session Validator"
}

func (p *Plugin) Version() string {
	return "internal"
}

func (p *Plugin) Init() {
	p.cpsCounter = cpsCounter{
		counters: map[time.Time]uint32{},
	}
}

func (p *Plugin) Load(cfg map[string]any) error {
	if err := config.Unmarshal(cfg, &p.config); err != nil {
		return err
	}

	if !p.config.SessionValidator.Enable {
		return infrared.ErrPluginViaConfigDisabled
	}

	validators, err := p.config.loadSessionValidatorConfigs()
	if err != nil {
		return err
	}

	clearMap := func(k any, v any) bool {
		p.validators.Delete(k)
		return true
	}
	p.validators.Range(clearMap)

	for k, v := range validators {
		p.validators.Store(k, v)
	}

	return nil
}

func (p *Plugin) Reload(cfg map[string]any) error {
	if p.eventBus == nil {
		return errors.New("")
	}

	if err := p.Load(cfg); err != nil {
		return err
	}

	cpsThreshold := p.config.Defaults.SessionValidator.CPSThreshold
	p.eventBus.DetachRecipient(p.eventID)
	p.eventID = p.eventBus.HandleFunc(p.onPlayerJoin(cpsThreshold), infrared.PlayerJoinEventTopic)
	return nil
}

func (p *Plugin) Enable(api infrared.PluginAPI) error {
	p.logger = api.Logger()
	p.eventBus = api.EventBus()

	cpsThreshold := p.config.Defaults.SessionValidator.CPSThreshold
	p.eventID = p.eventBus.HandleFunc(p.onPlayerJoin(cpsThreshold), infrared.PlayerJoinEventTopic)

	return nil
}

func (p *Plugin) Disable() error {
	p.eventBus.DetachRecipient(p.eventID)
	return nil
}

func (p *Plugin) onPlayerJoin(cpsThreshold int) event.HandlerSyncFunc {
	handleEvent := func(e event.Event) (any, error) {
		switch data := e.Data.(type) {
		case infrared.PlayerJoinEvent:
			player := data.Player
			serverID := data.Server.ID()
			if err := p.validatePlayer(player, serverID); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}

	if cpsThreshold <= 0 {
		return handleEvent
	}

	return func(e event.Event) (any, error) {
		p.cpsCounter.Inc()
		cps := p.cpsCounter.CPS()
		if cps < uint32(cpsThreshold) {
			return nil, nil
		}

		return handleEvent(e)
	}
}

func (p *Plugin) validatePlayer(player infrared.Player, serverID string) error {
	v, ok := p.validators.Load(serverID)
	if !ok {
		return nil
	}
	val := v.(validator)

	playerUUID, err := val.Validate(player)
	if err != nil {
		if errors.Is(err, errDisconnectAfterSuccessfullValidation) {
			return err
		}
		p.logger.Debug("failed to validate session",
			zap.Error(err),
		)
		return err
	}

	p.logger.Debug("validated player",
		zap.String("uuid", playerUUID.String()),
	)
	return nil
}

type validator interface {
	Validate(infrared.Player) (uuid.UUID, error)
}

type validatorFunc func(infrared.Player) (uuid.UUID, error)

func (fn validatorFunc) Validate(p infrared.Player) (uuid.UUID, error) {
	return fn(p)
}

var errDisconnectAfterSuccessfullValidation = errors.New("disconnect after successfull validation")

type storageValidator struct {
	storage             storage
	service             validator
	successDisconnector infrared.PlayerDisconnecter
	failureDisconnector infrared.PlayerDisconnecter
}

func (v storageValidator) Validate(player infrared.Player) (uuid.UUID, error) {
	playerUUID, err := v.storage.GetValidation(player.Username(), player.RemoteIP())
	if err != nil {
		if errors.Is(err, errValidationNotFound) {
			playerUUID, err = v.validate(player)
			if err == nil {
				v.successDisconnector.DisconnectPlayer(player, infrared.ApplyTemplates(
					infrared.TimeMessageTemplates(),
					infrared.PlayerMessageTemplates(player),
				))
				return playerUUID, errDisconnectAfterSuccessfullValidation
			}
		}
		v.failureDisconnector.DisconnectPlayer(player, infrared.ApplyTemplates(
			infrared.TimeMessageTemplates(),
			infrared.PlayerMessageTemplates(player),
		))
		return uuid.Nil, err
	}

	return playerUUID, nil
}

func (v storageValidator) validate(player infrared.Player) (uuid.UUID, error) {
	playerUUID, err := v.service.Validate(player)
	if err != nil {
		return uuid.Nil, err
	}

	if err := v.storage.PutValidation(player.Username(), player.RemoteIP(), playerUUID); err != nil {
		return uuid.Nil, err
	}

	return playerUUID, nil
}

type javaValidator struct {
	enc  java.SessionEncrypter
	auth java.SessionAuthenticator
}

func (jv javaValidator) Validator(serverID string, pubKey []byte) validator {
	return validatorFunc(func(player infrared.Player) (uuid.UUID, error) {
		switch p := player.(type) {
		case *java.Player:
			session, err := jv.validatePlayer(p, serverID, pubKey)
			if err != nil {
				return uuid.Nil, err
			}
			return session.PlayerUUID, nil
		default:
			return uuid.Nil, errors.New("not supported")
		}
	})
}

func (jv javaValidator) validatePlayer(player *java.Player, serverID string, pubKey []byte) (*java.Session, error) {
	verifyToken, err := jv.enc.GenerateVerifyToken()
	if err != nil {
		return nil, err
	}

	reqPk := login.ClientBoundEncryptionRequest{
		ServerID:    protocol.String(serverID),
		PublicKey:   pubKey,
		VerifyToken: verifyToken,
	}.Marshal()

	player.SetDeadline(time.Now().Add(time.Millisecond * 500))
	if err := player.WritePacket(reqPk); err != nil {
		return nil, err
	}

	respPk, err := player.ReadPacket(login.MaxSizeServerBoundEncryptionResponse)
	if err != nil {
		return nil, err
	}
	player.SetDeadline(time.Time{})

	resp, err := login.UnmarshalServerBoundEncryptionResponse(respPk)
	if err != nil {
		return nil, err
	}

	sharedSecret, err := jv.enc.DecryptAndVerifySharedSecret(verifyToken, resp.VerifyToken, resp.SharedSecret)
	if err != nil {
		return nil, err
	}

	if err := player.SetEncryption(sharedSecret); err != nil {
		return nil, err
	}

	sessionHash := java.GenerateSessionHash(serverID, sharedSecret, pubKey)
	return jv.auth.AuthenticateSession(player.Username(), sessionHash)
}
