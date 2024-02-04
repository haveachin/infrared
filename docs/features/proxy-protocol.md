# PROXY Protocol

Infrared supportes [PROXY Protocol v2](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt).

## Receive PROXY Protocol

You can receive PROXY Protocol Headers, but you **need** to specify your trusted [CIDRs](https://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing#IPv4_CIDR_blocks).
To enable it in Infrared you just have to change this in you [global config](../config/index):

```yml
# This is for receiving PROXY Protocol Headers
#
proxyProtocol:
  # Set this to true to enable it.
  # You also need to set trusted CIDRs to use this feature.
  # You can only receive PROXY Protocol Headers from trusted CIDRs.
  #
  receive: false
  
  # List all your trusted CIDRs here.
  # A CIDR is basically a way to talk about a whole range of IPs
  # instead of just one.
  #
  trustedCIDRs:
    - 127.0.0.1/32
```

## Forward Player IPs

You can forward the player IPs via PROXY Protocol.
To enable it in Infrared you just have to change this in you [**proxy config**](../config/proxies):
```yml
# Send a PROXY Protocol Header to the server to
# forward the players IP address.
#
#sendProxyProtocol: true // [!code --]
sendProxyProtocol: true // [!code ++]
```

## Paper

In Paper you have to enable it also to work.
See [the Paper documentation on PROXY Protocol](https://docs.papermc.io/paper/reference/global-configuration#proxies_proxy_protocol) for more.