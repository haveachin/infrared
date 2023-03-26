<p align="center">
  <img width="300" height="auto" src="assets/Infrared_Logo_2000x2000.png">
</p>

<div style="float: left;">

[![Discord](https://img.shields.io/discord/800456341088370698?label=discord&logo=discord)](https://discord.gg/r98YPRsZAx)
[![Docker Pulls](https://img.shields.io/docker/pulls/haveachin/infrared?logo=docker)](https://hub.docker.com/r/haveachin/infrared)

![Test, Build, Release](https://github.com/haveachin/infrared/actions/workflows/test-build-release.yml/badge.svg)

</div>

<div style="float: right;">
  <img height="60" src="assets/AGPLv3_Logo.svg"/>
</div>
<div style="clear: both;"/>

# Infrared - a Minecraft Proxy

An ultra lightweight Minecraft reverse proxy and status placeholder:
Ever wanted to have only one exposed port on your server for multiple Minecraft servers?
Then Infrared is the tool you need!
Infrared works as a reverse proxy using a sub-/domains to connect clients to a specific Minecraft server.

## Guides, Tutorials and Tips

- [Install](docs/INSTALL.md)
- [Usage](docs/USAGE.md)
- [Config](docs/CONFIG.md)

## Features

### Native

- [X] Reverse Proxy
  - [X] Wildcards Support
  - [X] Multi-Domain Support
- [X] Status Placeholder
  - [X] Override Online Status
  - [X] Offline Placeholder
- [X] Proxy Protocol v2 Support
- [X] RealIP Support
- [ ] TCPShield Plugin Support

### Internal Plugins

- [X] [Webhooks](docs/plugins/WEBHOOKS.md)
- [X] [HTTP REST API with JSON](docs/plugins/HTTP_API.md)
  - [X] Create/Read/Update/Delete Configs
  - [X] Query connected players
  - [X] Disconnect players
- [X] [Prometheus Analytics](docs/plugins/PROMETHEUS.md)
- [X] [Traffic Limiter](docs/plugins/TRAFFIC_LIMITER.md)
- [X] [Session Validator](docs/plugins/SESSION_VALIDATOR.md)

## Contributing

Feel free to add or modify the source code. On GitHub the best way of doing this is by forking this repository, then cloning your fork with Git to your local system. After adding or modifying the source code, push it back to your fork and open a pull request in this repository.

If you can't contribute by adding or modifying the source code, then you might be able to reach out to someone who can.
You can also contribute indirectly by donation.

## Coding Guidelines

## Project Layout

We try to use [golang-standards/project-layout](https://github.com/golang-standards/project-layout) as a reference. This should give Infrared a good This should give infrared a good foundation on which to grow. work to grow on.

### Commit Messages

When contributing to this project please follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) 
specification for writing commit messages, so that changelogs and release versions can be generated automatically.

Examples can be found here: https://www.conventionalcommits.org/en/v1.0.0/#examples

Some tooling that can help you author those commit messages are the following plugins:

* JetBrains Plugin [Conventional Commit](https://plugins.jetbrains.com/plugin/13389-conventional-commit)
  by [Edoardo Luppi](https://github.com/lppedd)
* Visual Studio
  Plugin [Conventional Commits](https://marketplace.visualstudio.com/items?itemName=vivaxy.vscode-conventional-commits)
  by [vivaxy](https://marketplace.visualstudio.com/publishers/vivaxy)

## Similar Projects

* https://github.com/itzg/mc-router

## Attributions

* <a href="https://commons.wikimedia.org/wiki/File:AGPLv3_Logo.svg">Free Software Foundation</a>, Public domain, via Wikimedia Commons