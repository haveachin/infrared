package infrared

import (
	"net"

	"go.uber.org/zap"
)

func logListener(l net.Listener) []zap.Field {
	return []zap.Field{
		zap.String("listenerNetwork", l.Addr().Network()),
		zap.String("listenerAddr", l.Addr().String()),
	}
}

func logConn(c net.Conn) []zap.Field {
	return []zap.Field{
		zap.String("connNetwork", c.LocalAddr().Network()),
		zap.String("connLocalAddr", c.LocalAddr().String()),
		zap.String("connRemoteAddr", c.RemoteAddr().String()),
	}
}

func logProcessedConn(pc ProcessedConn) []zap.Field {
	return []zap.Field{
		zap.String("connNetwork", pc.LocalAddr().Network()),
		zap.String("connLocalAddr", pc.LocalAddr().String()),
		zap.String("connRemoteAddr", pc.RemoteAddr().String()),
		zap.String("requestedServerAddr", pc.ServerAddr()),
		zap.String("username", pc.Username()),
		zap.String("gatewayId", pc.GatewayID()),
		zap.Bool("isLoginRequest", pc.IsLoginRequest()),
	}
}

func logServer(s Server) []zap.Field {
	return []zap.Field{
		zap.String("serverId", s.ID()),
		zap.Strings("serverDomains", s.Domains()),
	}
}
