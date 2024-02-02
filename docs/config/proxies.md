# Proxy

All proxy configs should live in the `proxies` directory.
The proxy directory can be changed via the [Proxies Path](cli-and-env-vars#proxies-path)

Minimal proxy config example:
```yml [my-server.yml]
# This is the domain that players enter in their game client.
# You can have multiple domains here or just one.
# Currently this holds just a wildcard character as a domain
# meaning that is accepts every domain that a player uses.
# Supports '*' and '?' wildcards in the pattern string.
#
domains:
  - "example.com"

addresses:
  - 127.0.0.1:25565
```

[Complete proxy config example](https://github.com/haveachin/infrared/blob/main/configs/proxy.yml)