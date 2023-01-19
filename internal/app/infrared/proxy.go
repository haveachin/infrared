package infrared

import (
	"time"

	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type ProxyConfig interface {
	ListenerBuilder() ListenerBuilder
	LoadGateways() ([]Gateway, error)
	LoadServers() ([]Server, error)
	LoadConnProcessor() (ConnProcessor, error)
	LoadProxySettings() (ProxySettings, error)
	LoadMiddlewareSettings() (MiddlewareSettings, error)
}

type ProxyChannelCaps struct {
	ConnProcessor int
	Server        int
	ConnPool      int
}

type ProxySettings struct {
	ChannelCaps ProxyChannelCaps
	CPNCount    int
}

type MiddlewareSettings struct {
	RateLimiter *RateLimiterSettings
}

type RateLimiterSettings struct {
	RequestLimit int
	WindowLength time.Duration
}

type Proxy interface {
	Reload(cfg ProxyConfig) error
	ListenAndServe(bus event.Bus, logger *zap.Logger)
	Close() error

	Players() []Player
	PlayerCount() int
}

type proxy struct {
	listenersManager ListenersManager
	settings         ProxySettings
	gateways         []Gateway
	cpnPool          CPNPool
	serverGateway    ServerGateway
	connPool         ConnPool
	cpnCh            chan Conn
	srvCh            chan Player
	poolCh           chan ConnTunnel
	logger           *zap.Logger
	eventBus         event.Bus
}

func NewProxy(cfg ProxyConfig) (*proxy, error) {
	gws, err := cfg.LoadGateways()
	if err != nil {
		return nil, err
	}

	cp, err := cfg.LoadConnProcessor()
	if err != nil {
		return nil, err
	}

	srvs, err := cfg.LoadServers()
	if err != nil {
		return nil, err
	}

	stg, err := cfg.LoadProxySettings()
	if err != nil {
		return nil, err
	}

	mwStg, err := cfg.LoadMiddlewareSettings()
	if err != nil {
		return nil, err
	}

	mws := []func(Handler) Handler{}
	if mwStg.RateLimiter != nil {
		stg := mwStg.RateLimiter
		mws = append(mws, RateLimitByIP(stg.RequestLimit, stg.WindowLength))
	}

	chCaps := stg.ChannelCaps
	cpnCh := make(chan Conn, chCaps.ConnProcessor)
	srvCh := make(chan Player, chCaps.Server)
	poolCh := make(chan ConnTunnel, chCaps.ConnPool)
	return &proxy{
		listenersManager: ListenersManager{
			New:       cfg.ListenerBuilder(),
			listeners: map[string]*managedListener{},
		},
		settings: stg,
		gateways: gws,
		cpnPool: CPNPool{
			CPN: CPN{
				ConnProcessor: cp,
				In:            cpnCh,
				Out:           srvCh,
				Middlewares:   mws,
			},
		},
		serverGateway: ServerGateway{
			ServerGatewayConfig: ServerGatewayConfig{
				Gateways: gws,
				Servers:  srvs,
				InChan:   srvCh,
				OutChan:  poolCh,
			},
		},
		connPool: ConnPool{
			ConnPoolConfig: ConnPoolConfig{
				In: poolCh,
			},
		},
		cpnCh:  cpnCh,
		srvCh:  srvCh,
		poolCh: poolCh,
	}, nil
}

func (p *proxy) setEventBus(bus event.Bus) {
	p.eventBus = bus

	for _, gw := range p.gateways {
		gw.SetEventBus(bus)
	}

	p.cpnPool.CPN.EventBus = bus
	p.connPool.EventBus = bus
	p.serverGateway.EventBus = bus
}

func (p *proxy) setLogger(logger *zap.Logger) {
	p.logger = logger
	p.listenersManager.logger = logger

	for _, gw := range p.gateways {
		gw.SetLogger(logger)
	}

	p.cpnPool.CPN.Logger = logger
	p.serverGateway.Logger = logger
	p.connPool.Logger = logger
}

func (p *proxy) ListenAndServe(bus event.Bus, logger *zap.Logger) {
	p.setEventBus(bus)
	p.setLogger(logger)
	p.cpnPool.SetSize(p.settings.CPNCount)

	for _, gw := range p.gateways {
		gw.SetListenersManager(&p.listenersManager)
		go ListenAndServe(gw, p.cpnCh)
	}

	go p.connPool.Start()
	p.serverGateway.Start()
}

func (p *proxy) Reload(cfg ProxyConfig) error {
	np, err := NewProxy(cfg)
	if err != nil {
		return err
	}
	np.setLogger(p.logger)
	np.setEventBus(p.eventBus)

	for _, gw := range p.gateways {
		gw.Close()
	}
	p.cpnPool.Close()

	p.gateways = np.gateways
	p.settings = np.settings

	p.cpnPool.SetSize(0)
	p.cpnPool.CPN = np.cpnPool.CPN
	p.cpnPool.SetSize(p.settings.CPNCount)

	p.serverGateway.Reload(np.serverGateway.ServerGatewayConfig)
	p.connPool.Reload(np.connPool.ConnPoolConfig)

	p.swapCPNChan(np.cpnCh)
	p.swapSrvChan(np.srvCh)
	p.swapPoolChan(np.poolCh)

	for _, gw := range p.gateways {
		gw.SetListenersManager(&p.listenersManager)
		gw.SetLogger(p.logger)
		go ListenAndServe(gw, p.cpnCh)
	}
	p.listenersManager.prune()

	return nil
}

func (p *proxy) swapCPNChan(cpnCh chan Conn) {
	close(p.cpnCh)
	for c := range p.cpnCh {
		cpnCh <- c
	}
	p.cpnCh = cpnCh
}

func (p *proxy) swapSrvChan(srvCh chan Player) {
	close(p.srvCh)
	for c := range p.srvCh {
		srvCh <- c
	}
	p.srvCh = srvCh
}

func (p *proxy) swapPoolChan(poolCh chan ConnTunnel) {
	close(p.poolCh)
	for c := range p.poolCh {
		poolCh <- c
	}
	p.poolCh = poolCh
}

func (p *proxy) Close() error {
	var err error
	for _, gw := range p.gateways {
		err = multierr.Append(err, gw.Close())
	}
	err = multierr.Append(err, p.serverGateway.Close())
	p.cpnPool.Close()
	close(p.cpnCh)
	close(p.srvCh)
	close(p.poolCh)
	return err
}

func (p *proxy) Players() []Player {
	p.connPool.mu.Lock()
	defer p.connPool.mu.Unlock()

	pp := make([]Player, 0, len(p.connPool.pool))
	for _, ct := range p.connPool.pool {
		pp = append(pp, ct.Conn)
	}
	return pp
}

func (p *proxy) PlayerCount() int {
	p.connPool.mu.Lock()
	defer p.connPool.mu.Unlock()

	return len(p.connPool.pool)
}
