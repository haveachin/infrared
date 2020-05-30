package infrared

import "errors"

var (
	ErrNoGateInGateway                 = errors.New("no gate in gateway")
	ErrNoProxyInGate                   = errors.New("no proxy in gate")
	ErrProxyNotSupported               = errors.New("proxy not supported in this gate")
	ErrProxySignatureAlreadyRegistered = errors.New("proxy signature is already registered in this gate")
	ErrGateSignatureAlreadyRegistered  = errors.New("gate signature is already registered in this gateway")
	ErrGateDoesNotExist                = errors.New("gate does not exist in this gateway")
)
