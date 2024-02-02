# Config

On fist start Infrared should generate a `config.yml` file and a `proxies` directory.
Here is a minimal `config.yml` example:

```yml
# Minimal Infrared Config

# Address that Infrared bind and listens to
#
bind: 0.0.0.0:25565

# Maximum duration between packets before the client gets timed out.
#
keepAliveTimeout: 30s
```

[Complete config example](https://github.com/haveachin/infrared/blob/main/configs/config.yml)