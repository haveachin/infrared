package infrared_test

import (
	"bufio"
	"errors"
	"net"
	"testing"

	ir "github.com/haveachin/infrared/pkg/infrared"
	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/haveachin/infrared/pkg/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/pkg/infrared/protocol/login"
	"github.com/pires/go-proxyproto"
)

type VirtualConn struct {
	net.Conn
	remoteAddr net.Addr
}

func (c VirtualConn) RemoteAddr() net.Addr {
	if c.remoteAddr == nil {
		c.remoteAddr = &net.TCPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: 25565,
		}
	}

	return c.remoteAddr
}

func (c VirtualConn) SendProxyProtocolHeader() error {
	header := &proxyproto.Header{
		Version:           2,
		Command:           proxyproto.PROXY,
		TransportProtocol: proxyproto.TCPv4,
		SourceAddr:        c.RemoteAddr(),
		DestinationAddr:   c.RemoteAddr(),
	}

	if _, err := header.WriteTo(c); err != nil {
		return err
	}

	return nil
}

func (c VirtualConn) SendHandshake(hs handshaking.ServerBoundHandshake) error {
	pk := protocol.Packet{}
	if err := hs.Marshal(&pk); err != nil {
		return err
	}

	_, err := pk.WriteTo(c.Conn)
	return err
}

func (c VirtualConn) SendLoginStart(ls login.ServerBoundLoginStart, v protocol.Version) error {
	pk := protocol.Packet{}
	if err := ls.Marshal(&pk, v); err != nil {
		return err
	}

	_, err := pk.WriteTo(c.Conn)
	return err
}

type VirtualListener struct {
	connChan         <-chan net.Conn
	errChan          chan error
	acceptTickerChan chan struct{}
}

func (l *VirtualListener) Accept() (net.Conn, error) {
	l.errChan = make(chan error)

	l.acceptTickerChan <- struct{}{}
	defer func() {
		select {
		case <-l.acceptTickerChan:
		default:
		}
	}()

	select {
	case c := <-l.connChan:
		return c, nil
	case err := <-l.errChan:
		return nil, err
	}
}

func (l *VirtualListener) AcceptTick() <-chan struct{} {
	return l.acceptTickerChan
}

func (l *VirtualListener) Close() error {
	l.errChan <- net.ErrClosed
	return nil
}

func (l *VirtualListener) Addr() net.Addr {
	return nil
}

type VirtualInfrared struct {
	vir      *ir.Infrared
	vl       *VirtualListener
	connChan chan<- net.Conn
}

func (vi *VirtualInfrared) ListenAndServe() error {
	return vi.vir.ListenAndServe()
}

func (vi *VirtualInfrared) MustListenAndServe(t *testing.T) {
	if err := vi.ListenAndServe(); errors.Is(err, net.ErrClosed) {
		return
	} else if err != nil {
		t.Error(err)
	}
}

func (vi *VirtualInfrared) NewConn(remoteAddr net.Addr) VirtualConn {
	cIn, cOut := net.Pipe()
	vi.connChan <- VirtualConn{
		Conn:       cOut,
		remoteAddr: remoteAddr,
	}
	return VirtualConn{Conn: cIn}
}

func (vi *VirtualInfrared) Close() {
	_ = vi.vl.Close()
}

func (vi *VirtualInfrared) AcceptTick() <-chan struct{} {
	return vi.vl.AcceptTick()
}

// NewVirtualInfrared sets up a virtualized Infrared instance that is ready to accept new virutal connections.
// Connections are simulated via synchronous, in-memory, full duplex network connection (see net.Pipe).
// It returns a the virtual Infrared instance and the output pipe to the virutal external server.
// Use the out pipe to see what is actually sent to the server. Like the PROXY Protocol header.
func NewVirtualInfrared(
	cfg ir.Config,
	sendProxyProtocol bool,
) (*VirtualInfrared, net.Conn) {
	vir := ir.NewWithConfig(cfg)

	connChan := make(chan net.Conn)
	vl := &VirtualListener{
		connChan:         connChan,
		errChan:          make(chan error),
		acceptTickerChan: make(chan struct{}, 1),
	}
	vir.NewListenerFunc = func(addr string) (net.Listener, error) {
		return vl, nil
	}

	rcIn, rcOut := net.Pipe()
	rc := VirtualConn{Conn: rcIn}
	vir.NewServerRequesterFunc = func(s []*ir.Server) (ir.ServerRequester, error) {
		return ir.ServerRequesterFunc(func(sr ir.ServerRequest) (ir.ServerResponse, error) {
			return ir.ServerResponse{
				ServerConn:        ir.NewServerConn(&rc),
				SendProxyProtocol: sendProxyProtocol,
			}, nil
		}), nil
	}

	return &VirtualInfrared{
		vir:      vir,
		vl:       vl,
		connChan: connChan,
	}, rcOut
}

func TestInfrared_SendProxyProtocol_True(t *testing.T) {
	vi, srvOut := NewVirtualInfrared(ir.NewConfig(), true)
	go vi.MustListenAndServe(t)

	vc := vi.NewConn(nil)
	if err := vc.SendHandshake(handshaking.ServerBoundHandshake{}); err != nil {
		t.Fatal(err)
	}
	if err := vc.SendLoginStart(login.ServerBoundLoginStart{}, protocol.Version1_20_2); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(srvOut)
	header, err := proxyproto.Read(r)
	if err != nil {
		t.Fatalf("Unexpected error reading proxy protocol header: %v", err)
	}

	if header.Command != proxyproto.PROXY {
		t.Fatalf("Unexpected proxy protocol command: %v", header.Command)
	}

	if header.TransportProtocol != proxyproto.TCPv4 {
		t.Fatalf("Unexpected proxy protocol transport protocol: %v", header.TransportProtocol)
	}

	if header.Version != 2 {
		t.Fatalf("Unexpected proxy protocol version: %v", header.Version)
	}
}

func TestInfrared_SendProxyProtocol_False(t *testing.T) {
	vi, srvOut := NewVirtualInfrared(ir.NewConfig(), false)
	go vi.MustListenAndServe(t)

	vc := vi.NewConn(nil)
	if err := vc.SendHandshake(handshaking.ServerBoundHandshake{}); err != nil {
		t.Fatal(err)
	}
	if err := vc.SendLoginStart(login.ServerBoundLoginStart{}, protocol.Version1_20_2); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(srvOut)
	if _, err := proxyproto.Read(r); err == nil {
		t.Fatal("Expected error reading proxy protocol header, but got nothing")
	}
}

func TestInfrared_ReceiveProxyProtocol_True(t *testing.T) {
	cfg := ir.NewConfig().
		WithProxyProtocolReceive(true).
		WithProxyProtocolTrustedCIDRs("127.0.0.1/32")

	vi, _ := NewVirtualInfrared(cfg, false)
	go vi.MustListenAndServe(t)

	vc := vi.NewConn(nil)
	if err := vc.SendProxyProtocolHeader(); err != nil {
		t.Fatal(err)
	}
	if err := vc.SendHandshake(handshaking.ServerBoundHandshake{}); err != nil {
		t.Fatal(err)
	}
	if err := vc.SendLoginStart(login.ServerBoundLoginStart{}, protocol.Version1_20_2); err != nil {
		t.Fatal(err)
	}
}

func TestInfrared_ReceiveProxyProtocol_False(t *testing.T) {
	cfg := ir.NewConfig().
		WithProxyProtocolReceive(false)

	vi, _ := NewVirtualInfrared(cfg, false)
	go vi.MustListenAndServe(t)

	vc := vi.NewConn(nil)
	if err := vc.SendProxyProtocolHeader(); err != nil {
		t.Fatal(err)
	}
	if err := vc.SendHandshake(handshaking.ServerBoundHandshake{}); err != nil {
		return
	}
	t.Fatal("no disconnect after invalid proxy protocol header")
}

func TestInfrared_ReceiveProxyProtocol_True_ErrNoTrustedCIDRs(t *testing.T) {
	cfg := ir.NewConfig().
		WithProxyProtocolReceive(true).
		WithProxyProtocolTrustedCIDRs()

	vi, _ := NewVirtualInfrared(cfg, false)

	errChan := make(chan error, 1)
	go func() {
		errChan <- vi.ListenAndServe()
	}()

	select {
	case err := <-errChan:
		if !errors.Is(err, ir.ErrNoTrustedCIDRs) {
			t.Fatalf("got: %s; want: %s", err, ir.ErrNoTrustedCIDRs)
		}
	case <-vi.AcceptTick():
		vi.Close()
		t.Fatalf("got: no error during init; want: %s", ir.ErrNoTrustedCIDRs)
	}
}

func TestInfrared_ReceiveProxyProtocol_True_UntrustedIP(t *testing.T) {
	cfg := ir.NewConfig().
		WithProxyProtocolReceive(true).
		WithProxyProtocolTrustedCIDRs("127.0.0.1/32")

	vi, _ := NewVirtualInfrared(cfg, false)
	go vi.MustListenAndServe(t)

	vc := vi.NewConn(&net.TCPAddr{
		IP:   net.IPv4(12, 34, 56, 78),
		Port: 12345,
	})

	if err := vc.SendProxyProtocolHeader(); err == nil {
		t.Fatal("no disconnect after untrusted IP")
	}
}
