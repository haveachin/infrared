<p align="center">
   <img width="300" height="auto" src="https://i.imgur.com/sD8cjJc.png">
 </p>

[![Discord](https://img.shields.io/discord/800456341088370698?label=discord&logo=discord)](https://discord.gg/r98YPRsZAx)
[![Docker Pulls](https://img.shields.io/docker/pulls/haveachin/infrared?logo=docker)](https://hub.docker.com/r/haveachin/infrared)

![build](https://github.com/haveachin/infrared/actions/workflows/test.yml/badge.svg)
[![GitHub](https://img.shields.io/github/license/haveachin/infrared)](https://raw.githubusercontent.com/haveachin/infrared/master/LICENSE)

# Infrared - a Minecraft Proxy

An ultra lightweight Minecraft reverse proxy and status placeholder:
Ever wanted to have only one exposed port on your server for multiple Minecraft servers?
Then Infrared is the tool you need!
Infrared works as a reverse proxy using a sub-/domains to connect clients to a specific Minecraft server.

## Features

### Native

- [X] Reverse Proxy
  - [X] Wildcards Support
  - [X] Mult-Domain Support
- [X] Status Placeholder
  - [X] Override Online Status
  - [X] Offline Placeholder
- [X] HAProxy Protocol Support
- [X] RealIP Support
- [ ] TCPShield Plugin Support

### Internal Plugins

- [X] [Webhooks](docs/plugins/WEBHOOKS.md)
- [ ] [HTTP REST API with JSON](docs/plugins/HTTP_API.md)
  - [ ] Create/Read/Update/Delete Configs
  - [ ] Query connected players
  - [ ] Disconnect players
- [ ] [Prometheus Analytics](docs/plugins/PROMETEUS.md)
- [ ] Server Hibernation

## How to configure

- [Usage](docs/USAGE.md)
- [Config](docs/CONFIG.md)

## Similar Projects

* https://github.com/itzg/mc-router
