# Proxy

All proxy configs should live in the `proxies` directory.
The proxy directory can be changed via the [Proxies Path](cli-and-env-vars#proxies-path)

Proxy config example:
```yml [my-server.yml]
# This is the domain that players enter in their game client.
# You can have multiple domains here or just one.
# Currently this holds just a wildcard character as a domain
# meaning that is accepts every domain that a player uses.
# Supports '*' and '?' wildcards in the pattern string.
#
domains:
  - "*"

addresses:
  - example.com:25565

# Send a Proxy Protocol v2 Header to the server to
# forward the players IP address
#
#sendProxyProtocol: true
```