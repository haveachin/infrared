<p align="center">
  <img width="200" height="auto" src="docs/public/img/logo.svg">
</p>
<h1 align="center"><b>Infrared</b></h1>
<h3 align="center"><b>A Minecraft Reverse Proxy</b></h3>

<p align="center">
  <a href="https://discord.gg/r98YPRsZAx">
  <img alt="Discord" src="https://img.shields.io/discord/800456341088370698?label=discord&logo=discord" />
  </a>
  <a href="https://hub.docker.com/r/haveachin/infrared">
  <img alt="Docker Pulls" src="https://img.shields.io/docker/pulls/haveachin/infrared?logo=docker" />
  </a>
  <br />
  <img alt="CI" src="https://github.com/haveachin/infrared/actions/workflows/ci.yml/badge.svg" />
</p>

> [!WARNING] 
> Infrared is currently under active development: bugs and breaking changes can happen.
> Feedback and contributions are welcome.

An ultra lightweight Minecraft reverse proxy and status placeholder:
Ever wanted to have only one exposed port on your server for multiple Minecraft servers?
Then Infrared is the tool you need!
Infrared works as a reverse proxy using a sub-/domains to connect clients to a specific Minecraft server.

## Features

- [X] Reverse Proxy
  - [X] Wildcards Support
  - [X] Multi-Domain Support
- [X] Status Response Caching
- [X] Proxy Protocol Support
- [X] Ratelimiter

## Useful Links

- **[Docs](https://infrared.dev)**
- **[Ask Questions](https://github.com/haveachin/infrared/discussions)**
- [Latest Release](https://github.com/haveachin/infrared/releases/latest)
- [Discord Invite](https://discord.gg/r98YPRsZAx)
- [Contributing](CONTRIBUTING.md)

## Build

Requirements:
- [Go](https://go.dev/) 1.21+

```
CGO_ENABLED=0 go build -ldflags "-s -w" -o ./out/infrared ./cmd/infrared
```
or `make all` (requires GNU Make). The binary is in the `out/` directory.

## Similar Projects

* https://github.com/itzg/mc-router

## Attributions

- [Free Software Foundation](https://commons.wikimedia.org/wiki/File:AGPLv3_Logo.svg), Public domain, via Wikimedia Commons
- [Tnze/go-mc](https://github.com/Tnze/go-mc) 🚀, MIT
- [IGLOU-EU/go-wildcard](https://github.com/IGLOU-EU/go-wildcard), Apache-2.0
- [cespare/xxhash](https://github.com/cespare/xxhash), MIT
- [google/uuid](https://github.com/google/uuid), BSD-3-Clause
- [pires/go-proxyproto](https://github.com/pires/go-proxyproto), Apache-2.0
- [spf13/pflag](https://github.com/spf13/pflag), BSD-3-Clause
- [go-yaml/yaml](https://github.com/go-yaml/yaml), Apache-2.0, MIT
- [vitepress](https://github.com/vuejs/vitepress), MIT
- [tollbooth](https://github.com/didip/tollbooth), MIT

<br />
<p align="center">
  <img height="60" src="docs/public/img/agplv3_logo.svg"/>
</p>
