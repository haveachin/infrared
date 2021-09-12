package infrared

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/login"
	"github.com/haveachin/infrared/protocol/status"
)

type PlayerSample struct {
	Name string
	UUID string
}

type StatusResponse struct {
	VersionName    string
	ProtocolNumber int
	MaxPlayers     int
	PlayersOnline  int
	PlayerSamples  []PlayerSample
	IconPath       string
	MOTD           string

	cachedJSON *status.ResponseJSON
}

func (resp StatusResponse) ResponseJSON() (status.ResponseJSON, error) {
	if resp.cachedJSON != nil {
		return *resp.cachedJSON, nil
	}

	var samples []status.PlayerSampleJSON
	for _, sample := range resp.PlayerSamples {
		samples = append(samples, status.PlayerSampleJSON{
			Name: sample.Name,
			ID:   sample.UUID,
		})
	}

	img64, err := loadImageAndEncodeToBase64String(resp.IconPath)
	if err != nil {
		return status.ResponseJSON{}, err
	}

	resp.cachedJSON = &status.ResponseJSON{
		Version: status.VersionJSON{
			Name:     resp.VersionName,
			Protocol: resp.ProtocolNumber,
		},
		Players: status.PlayersJSON{
			Max:    resp.MaxPlayers,
			Online: resp.PlayersOnline,
			Sample: samples,
		},
		Favicon: img64,
		Description: status.DescriptionJSON{
			Text: resp.MOTD,
		},
	}

	return *resp.cachedJSON, nil
}

func loadImageAndEncodeToBase64String(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	imgFile, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer imgFile.Close()

	bb, err := io.ReadAll(imgFile)
	if err != nil {
		return "", err
	}
	img64 := base64.StdEncoding.EncodeToString(bb)

	return fmt.Sprintf("data:image/png;base64,%s", img64), nil
}

type Server struct {
	ID                string
	Domains           []string
	Address           string
	SendProxyProtocol bool
	SendRealIP        bool
	DisconnectMessage string
	OnlineStatus      StatusResponse
	OfflineStatus     StatusResponse
	WebhookIDs        []string
}

func (srv Server) Dial() (Conn, error) {
	c, err := net.DialTimeout("tcp", srv.Address, 1*time.Second)
	if err != nil {
		return nil, err
	}

	return newConn(c), nil
}

func (srv Server) replaceTemplates(c ProcessingConn, msg string) string {
	tmpls := map[string]string{
		"username":      c.username,
		"now":           time.Now().Format(time.RFC822),
		"remoteAddress": c.RemoteAddr().String(),
		"localAddress":  c.LocalAddr().String(),
		"domain":        c.srvHost,
		"serverAddress": srv.Address,
	}

	for k, v := range tmpls {
		msg = strings.Replace(msg, fmt.Sprintf("{{%s}}", k), v, -1)
	}

	return msg
}

func (srv Server) handleOfflineStatusRequest(c ProcessingConn) error {
	respJSON, err := srv.OfflineStatus.ResponseJSON()
	if err != nil {
		return err
	}

	bb, err := json.Marshal(respJSON)
	if err != nil {
		return err
	}

	respPk := status.ClientBoundResponse{
		JSONResponse: protocol.String(bb),
	}.Marshal()

	if err := c.WritePacket(respPk); err != nil {
		return err
	}

	pingPk, err := c.ReadPacket()
	if err != nil {
		return err
	}

	return c.WritePacket(pingPk)
}

func (srv Server) handleOfflineLoginRequest(c ProcessingConn) error {
	msg := srv.replaceTemplates(c, srv.DisconnectMessage)

	pk := login.ClientBoundDisconnect{
		Reason: protocol.Chat(fmt.Sprintf("{\"text\":\"%s\"}", msg)),
	}.Marshal()

	return c.WritePacket(pk)
}

func (srv Server) handleOffline(c ProcessingConn) error {
	if c.handshake.IsStatusRequest() {
		return srv.handleOfflineStatusRequest(c)
	}

	return srv.handleOfflineLoginRequest(c)
}

func (srv Server) overrideStatusResponse(c ProcessingConn, rc Conn) error {
	pk, err := rc.ReadPacket()
	if err != nil {
		return err
	}

	respPk, err := status.UnmarshalClientBoundResponse(pk)
	if err != nil {
		return err
	}

	var respJSON status.ResponseJSON
	if err := json.Unmarshal([]byte(respPk.JSONResponse), &respJSON); err != nil {
		return err
	}

	motd := srv.replaceTemplates(c, srv.OnlineStatus.MOTD)
	respJSON.Description.Text = motd

	bb, err := json.Marshal(respJSON)
	if err != nil {
		return err
	}

	respPk.JSONResponse = protocol.String(bb)

	if err := c.WritePacket(respPk.Marshal()); err != nil {
		return err
	}

	return nil
}

func (srv Server) ProcessConnection(c ProcessingConn) (ProcessedConn, error) {
	rc, err := srv.Dial()
	if err != nil {
		if err := srv.handleOffline(c); err != nil {
			return ProcessedConn{}, err
		}
		return ProcessedConn{}, err
	}

	if err := rc.WritePackets(c.readPks...); err != nil {
		rc.Close()
		return ProcessedConn{}, err
	}

	if c.handshake.IsStatusRequest() {
		if err := srv.overrideStatusResponse(c, rc); err != nil {
			rc.Close()
			return ProcessedConn{}, err
		}
	}

	return ProcessedConn{
		ProcessingConn: c,
		ServerConn:     rc,
	}, nil
}

type ServerGateway struct {
	Servers []Server
	srvs    map[string]*Server
}

func (gw *ServerGateway) mapServers() {
	gw.srvs = map[string]*Server{}

	for _, server := range gw.Servers {
		for _, host := range server.Domains {
			hostLower := strings.ToLower(host)
			gw.srvs[hostLower] = &server
		}
	}
}

func (gw ServerGateway) Start(srvChan <-chan ProcessingConn, poolChan chan<- ProcessedConn) {
	gw.mapServers()

	for {
		c, ok := <-srvChan
		if !ok {
			break
		}

		hostLower := strings.ToLower(c.srvHost)
		log.Printf("[srvgateway|i] %s host=%s\n", c.RemoteAddr(), hostLower)
		srv, ok := gw.srvs[hostLower]
		if !ok {
			continue
		}

		log.Printf("[server|>] %s host=%s\n", c.RemoteAddr(), hostLower)
		pc, err := srv.ProcessConnection(c)
		if err != nil {
			continue
		}
		poolChan <- pc
	}
}
