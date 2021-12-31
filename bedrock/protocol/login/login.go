package login

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/golang-jwt/jwt/v4"
)

// chain holds a chain with claims, each with their own headers, payloads and signatures. Each claim holds
// a public key used to verify other claims.
type chain []string

// request is the outer encapsulation of the request. It holds a chain and a ClientData object.
type request struct {
	// Chain is the client certificate chain. It holds several claims that the server may verify in order to
	// make sure that the client is logged into XBOX Live.
	Chain chain `json:"chain"`
	// RawToken holds the raw token that follows the JWT chain, holding the ClientData.
	RawToken string `json:"-"`
}

func Parse(request []byte) (IdentityData, ClientData, error) {
	req, err := parseLoginRequest(request)
	if err != nil {
		return IdentityData{}, ClientData{}, fmt.Errorf("parse login request: %w", err)
	}

	jwtParser := jwt.Parser{}
	var identityClaims identityClaims
	switch len(req.Chain) {
	case 1:
		// Player was not authenticated with XBOX Live, meaning the one token in here is self-signed.
		_, _, err = jwtParser.ParseUnverified(req.Chain[2], &identityClaims)
		if err != nil {
			return IdentityData{}, ClientData{}, err
		}
		if err := identityClaims.Valid(); err != nil {
			return IdentityData{}, ClientData{}, fmt.Errorf("validate token 0: %w", err)
		}
	case 3:
		// Player was (or should be) authenticated with XBOX Live, meaning the chain is exactly 3 tokens
		// long.
		var c jwt.RegisteredClaims
		_, _, err := jwtParser.ParseUnverified(req.Chain[0], &c)
		if err != nil {
			return IdentityData{}, ClientData{}, fmt.Errorf("parse token 0: %w", err)
		}

		_, _, err = jwtParser.ParseUnverified(req.Chain[1], &c)
		if err != nil {
			return IdentityData{}, ClientData{}, fmt.Errorf("parse token 1: %w", err)
		}
		_, _, err = jwtParser.ParseUnverified(req.Chain[2], &identityClaims)
		if err != nil {
			return IdentityData{}, ClientData{}, fmt.Errorf("parse token 2: %w", err)
		}
	default:
		return IdentityData{}, ClientData{}, fmt.Errorf("unexpected login chain length %v", len(req.Chain))
	}

	var cData ClientData
	_, _, err = jwtParser.ParseUnverified(req.RawToken, &cData)
	if err != nil {
		return IdentityData{}, cData, fmt.Errorf("parse client data: %w", err)
	}

	return identityClaims.ExtraData, cData, nil
}

// parseLoginRequest parses the structure of a login request from the data passed and returns it.
func parseLoginRequest(requestData []byte) (*request, error) {
	buf := bytes.NewBuffer(requestData)
	chain, err := decodeChain(buf)
	if err != nil {
		return nil, err
	}
	if len(chain) < 1 {
		return nil, fmt.Errorf("JWT chain must be at least 1 token long")
	}
	var rawLength int32
	if err := binary.Read(buf, binary.LittleEndian, &rawLength); err != nil {
		return nil, fmt.Errorf("error reading raw token length: %v", err)
	}
	return &request{Chain: chain, RawToken: string(buf.Next(int(rawLength)))}, nil
}

// decodeChain reads a certificate chain from the buffer passed and returns each claim found in the chain.
func decodeChain(buf *bytes.Buffer) (chain, error) {
	var chainLength int32
	if err := binary.Read(buf, binary.LittleEndian, &chainLength); err != nil {
		return nil, fmt.Errorf("error reading chain length: %v", err)
	}
	chainData := buf.Next(int(chainLength))

	request := &request{}
	if err := json.Unmarshal(chainData, request); err != nil {
		return nil, fmt.Errorf("error decoding request chain JSON: %v", err)
	}
	// First check if the chain actually has any elements in it.
	if len(request.Chain) == 0 {
		return nil, fmt.Errorf("connection request had no claims in the chain")
	}
	return request.Chain, nil
}

// identityClaims holds the claims for the last token in the chain, which contains the IdentityData of the
// player.
type identityClaims struct {
	jwt.RegisteredClaims

	// ExtraData holds the extra data of this claim, which is the IdentityData of the player.
	ExtraData IdentityData `json:"extraData"`

	IdentityPublicKey string `json:"identityPublicKey"`
}
