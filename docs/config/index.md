# Config

On fist start Infrared should generate a `config.yml` file and a `proxies` directory.
A minmal

```yml
# Minimal Infrared Config

# Address that Infrared bind and listens to
#
bind: 0.0.0.0:25565

# Maximum duration between packets before the client gets timed out.
#
keepAliveTimeout: 30s
```
