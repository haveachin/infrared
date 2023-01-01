package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gofrs/uuid"
)

func MojangSessionServerURLHasJoined(username, sessionHash string) string {
	return fmt.Sprintf(
		"https://sessionserver.mojang.com/session/minecraft/hasJoined?username=%s&serverId=%s",
		username,
		sessionHash,
	)
}

func MojangSessionServerURLHasJoinedWithIP(username, sessionHash, ip string) string {
	return fmt.Sprintf("%s&ip=%s",
		MojangSessionServerURLHasJoined(username, sessionHash),
		ip,
	)
}

func GenerateSessionHash(serverID string, sharedSecret, publicKey []byte) string {
	notchHash := NewSha1Hash()
	notchHash.Update([]byte(serverID))
	notchHash.Update(sharedSecret)
	notchHash.Update(publicKey)
	return notchHash.HexDigest()
}

type PlayerSkin struct {
	Value     string
	Signature string
}

type Session struct {
	PlayerUUID uuid.UUID
	PlayerSkin PlayerSkin
}

type SessionAuthenticator interface {
	AuthenticateSession(username, sessionHash string) (Session, error)
	AuthenticateSessionPreventProxy(username, sessionHash, ip string) (Session, error)
}

type MojangSessionAuthenticator struct{}

func (auth *MojangSessionAuthenticator) AuthenticateSession(username, sessionHash string) (Session, error) {
	return auth.AuthenticateSessionPreventProxy(username, sessionHash, "")
}

func (auth *MojangSessionAuthenticator) AuthenticateSessionPreventProxy(username, sessionHash, ip string) (Session, error) {
	var url string
	if ip == "" {
		url = MojangSessionServerURLHasJoined(username, sessionHash)
	} else {
		url = MojangSessionServerURLHasJoinedWithIP(username, sessionHash, ip)
	}

	resp, err := http.Get(url)
	if err != nil {
		return Session{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return Session{}, fmt.Errorf("unable to authenticate session (%s)", resp.Status)
	}

	var p struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Properties []struct {
			Name      string `json:"name"`
			Value     string `json:"value"`
			Signature string `json:"signature"`
		} `json:"properties"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return Session{}, err
	}
	_ = resp.Body.Close()

	playerUUID, err := uuid.FromString(p.ID)
	if err != nil {
		return Session{}, err
	}

	var skinValue string
	var skinSignature string
	for _, property := range p.Properties {
		if property.Name == "textures" {
			skinValue = property.Value
			skinSignature = property.Signature
			break
		}
	}

	if skinValue == "" {
		return Session{}, errors.New("no skinValue in request")
	}

	return Session{
		PlayerUUID: playerUUID,
		PlayerSkin: PlayerSkin{
			Value:     skinValue,
			Signature: skinSignature,
		},
	}, nil
}
