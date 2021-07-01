package proxy

type ProxyLaneManager struct {
	// the key here is the listen address of the proxylane
	proxies map[string]ProxyLane
}
