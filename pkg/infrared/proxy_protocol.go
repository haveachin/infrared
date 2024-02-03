package infrared

import (
	"errors"
	"net"

	"github.com/pires/go-proxyproto"
)

var (
	ErrUpstreamNotTrusted = errors.New("upstream not trusted")
	ErrNoTrustedCIDRs     = errors.New("no trusted CIDRs")
)

type ProxyProtocolConfig struct {
	Receive      bool     `yaml:"receive"`
	TrustedCIDRs []string `yaml:"trustedCIDRs"`
}

func newProxyProtocolListener(l net.Listener, trustedCIDRs []string) (net.Listener, error) {
	if len(trustedCIDRs) == 0 {
		return nil, ErrNoTrustedCIDRs
	}

	cidrs := make([]*net.IPNet, len(trustedCIDRs))
	for i, trustedCIDR := range trustedCIDRs {
		_, cidr, err := net.ParseCIDR(trustedCIDR)
		if err != nil {
			return nil, err
		}
		cidrs[i] = cidr
	}

	return &proxyproto.Listener{
		Listener: l,
		Policy: func(upstream net.Addr) (proxyproto.Policy, error) {
			tcpAddr, ok := upstream.(*net.TCPAddr)
			if !ok {
				return proxyproto.REJECT, errors.New("not a tcp conn")
			}

			for _, cidr := range cidrs {
				if cidr.Contains(tcpAddr.IP) {
					return proxyproto.REQUIRE, nil
				}
			}

			return proxyproto.REJECT, ErrUpstreamNotTrusted
		},
	}, nil
}

func writeProxyProtocolHeader(addr net.Addr, rc net.Conn) error {
	rcAddr := rc.RemoteAddr()
	tcpAddr, ok := rcAddr.(*net.TCPAddr)
	if !ok {
		panic("not a tcp connection")
	}

	tp := proxyproto.TCPv4
	if tcpAddr.IP.To4() == nil {
		tp = proxyproto.TCPv6
	}

	header := &proxyproto.Header{
		Version:           2,
		Command:           proxyproto.PROXY,
		TransportProtocol: tp,
		SourceAddr:        addr,
		DestinationAddr:   rcAddr,
	}

	if _, err := header.WriteTo(rc); err != nil {
		return err
	}

	return nil
}
