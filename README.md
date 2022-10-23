<p align="center">
   <img width="300" height="auto" src="https://i.imgur.com/sD8cjJc.png">
 </p>

[![Discord](https://img.shields.io/discord/800456341088370698?label=discord&logo=discord)](https://discord.gg/r98YPRsZAx)
[![Docker Pulls](https://img.shields.io/docker/pulls/haveachin/infrared?logo=docker)](https://hub.docker.com/r/haveachin/infrared)

![build](https://github.com/haveachin/infrared/actions/workflows/test-build-release.yml/badge.svg)
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

- [Install](docs/INSTALL.md)
- [Usage](docs/USAGE.md)
- [Config](docs/CONFIG.md)

## Coding Guidelines

### Commit Messages

When contributing to this project please follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) 
specification for writing commit messages, so that changelogs and release versions can be generated automatically.

**Example commit message**

```
fix: prevent racing of requests

Introduce a request id and a reference to latest request. Dismiss
incoming responses other than from latest request.

Remove timeouts which were used to mitigate the racing issue but are
obsolete now.

Reviewed-by: Z
Refs: #123
```

Some tooling that can help you author those commit messages are the following plugins:

* JetBrains Plugin [Conventional Commit](https://plugins.jetbrains.com/plugin/13389-conventional-commit)
  by [Edoardo Luppi](https://github.com/lppedd)
* Visual Studio
  Plugin [Conventional Commits](https://marketplace.visualstudio.com/items?itemName=vivaxy.vscode-conventional-commits)
  by [vivaxy](https://marketplace.visualstudio.com/publishers/vivaxy)

## Similar Projects

* https://github.com/itzg/mc-router
