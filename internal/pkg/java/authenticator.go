package java

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/haveachin/infrared/internal/pkg/java/sha1"
)

func sessionServerURLHasJoined(baseURL, username, sessionHash string) string {
	return fmt.Sprintf(
		"%s/session/minecraft/hasJoined?username=%s&serverId=%s",
		baseURL,
		username,
		sessionHash,
	)
}

func sessionServerURLHasJoinedWithIP(baseURL, username, sessionHash, ip string) string {
	return fmt.Sprintf("%s&ip=%s",
		sessionServerURLHasJoined(baseURL, username, sessionHash),
		ip,
	)
}

func GenerateSessionHash(serverID string, sharedSecret, publicKey []byte) string {
	notchHash := sha1.NewHash()
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
	AuthenticateSession(username, sessionHash string) (*Session, error)
	AuthenticateSessionPreventProxy(username, sessionHash, ip string) (*Session, error)
}

type HTTPSessionAuthenticator struct {
	BaseURL string
}

func (auth HTTPSessionAuthenticator) AuthenticateSession(username, sessionHash string) (*Session, error) {
	return auth.AuthenticateSessionPreventProxy(username, sessionHash, "")
}

func (auth HTTPSessionAuthenticator) AuthenticateSessionPreventProxy(username, sessionHash, ip string) (*Session, error) {
	var url string
	if ip == "" {
		url = sessionServerURLHasJoined(auth.BaseURL, username, sessionHash)
	} else {
		url = sessionServerURLHasJoinedWithIP(auth.BaseURL, username, sessionHash, ip)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to authenticate session (%s)", resp.Status)
	}

	dto := &struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Properties []struct {
			Name      string `json:"name"`
			Value     string `json:"value"`
			Signature string `json:"signature"`
		} `json:"properties"`
	}{}

	if err := json.NewDecoder(resp.Body).Decode(dto); err != nil {
		return nil, err
	}

	playerUUID, err := uuid.FromString(dto.ID)
	if err != nil {
		return nil, err
	}

	skin := PlayerSkin{}
	for _, property := range dto.Properties {
		if property.Name == "textures" {
			skin.Value = property.Value
			skin.Signature = property.Signature
			break
		}
	}

	return &Session{
		PlayerUUID: playerUUID,
		PlayerSkin: skin,
	}, nil
}
