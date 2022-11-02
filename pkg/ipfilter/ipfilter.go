package ipfilter

import (
	"net"
	"sync"
)

type IPFilter interface {
	// Adds the IP to the filter if it wasn't already present.
	Add(ip net.IP)
	// Removes the IP from the filter if it was present.
	Remove(ip net.IP)
	IsAllowed(ip net.IP) bool
}

type Mode byte

const (
	ModeAllow Mode = iota
	ModeDeny
)

type ipFilter struct {
	// Maps IP as a string to net.IP
	ips  sync.Map
	mode Mode
}

func New(mode Mode) IPFilter {
	return &ipFilter{
		mode: mode,
	}
}

func (f ipFilter) Add(ip net.IP) {
	f.ips.Store(ip.String(), ip)
}

func (f ipFilter) Remove(ip net.IP) {
	f.ips.Delete(ip.String())
}

func (f ipFilter) IsAllowed(ip net.IP) bool {
	_, ok := f.ips.Load(ip.String())
	switch f.mode {
	case ModeAllow:
		return ok
	case ModeDeny:
		return !ok
	}
	return false
}
