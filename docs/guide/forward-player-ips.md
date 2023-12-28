# Forward Player IPs

You can forward the player IPs via proxy protocol.
To enable it in Infrared you just have to change this in you proxy config:
```yml
# Send a Proxy Protocol v2 Header to the server to
# forward the players IP address
#
#sendProxyProtocol: true // [!code --]
sendProxyProtocol: true // [!code ++]
```

## Paper

In Paper you have to enable it also to work. See [the Paper documentation on Proxy Protocol](https://docs.papermc.io/paper/reference/global-configuration#proxies_proxy_protocol) for more.