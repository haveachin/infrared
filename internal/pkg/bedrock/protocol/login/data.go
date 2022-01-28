package login

import "github.com/golang-jwt/jwt/v4"

// IdentityData contains identity data of the player logged in. It is found in one of the JWT claims signed
// by Mojang, and can thus be trusted.
type IdentityData struct {
	// DisplayName is the username of the player, which may be changed by the user. It should for that reason
	// not be used as a key to store information.
	DisplayName string `json:"displayName"`
}

// ClientData is a container of client specific data of a Login packet. It holds data such as the skin of a
// player, but also its language code and device information.
type ClientData struct {
	jwt.RegisteredClaims
	// ServerAddress is the exact address the player used to join the server with. This may be either an
	// actual address, or a hostname. ServerAddress also has the port in it, in the shape of
	// 'address:port`.
	ServerAddress string
}
