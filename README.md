<p align="center">
   <img width="300" height="auto" src="https://i.imgur.com/sD8cjJc.png">
 </p>




# Infrared

An ultra lightweight Minecraft reverse proxy and idle placeholder:
Ever wanted to have only one exposed port at your server for multiple Minecraft servers?
Then infrared is the tool you need!
Infrared works as a reverse proxy using a subdomains to connect clients to a specific Minecraft server.

## Features

- [x] Reverse Proxy
- [x] Display Placeholder Server
- [x] Autostart Server when pinged
- [x] Logger Callback URLs
- [ ] JSON Endpoint for logs
- [ ] gRPC API for live data

## Deploy

```shell script
$ docker build --no-cache -t haveachin/infrared:latest https://github.com/haveachin/infrared.git &&
  docker image prune -f --filter label=stage=intermediate &&
  docker run -d --name infrared --restart=unless-stopped -it -v /usr/local/infrared/configs/:/configs -p 25565:25565/tcp --expose 25565 haveachin/infrared:latest
```

## Update

```shell script
$ docker build --no-cache -t haveachin/infrared:latest https://github.com/haveachin/infrared.git &&
  docker image prune -f --filter label=stage=intermediate &&
  docker stop infrared &&
  docker rm infrared &&
  docker run -d --name infrared --restart=unless-stopped -it -v /usr/local/infrared/configs/:/configs -p 25565:25565/tcp --expose 25565 haveachin/infrared:latest
```

## Environment Variables

**Info**: Command-line flags override environment variables.

`INFRARED_DEBUG` enables debug logs [default: `false`]  
`INFRARED_COLOR` enables colorful logs [default: `true`]  
`INFRARED_CONFIG_PATH` is the path of all your server configs [default: `"./configs/"`]

## Command-Line Flags

`-debug` enables debug logs [default: `false`]  
`-color` enables colorful logs [default: `true`]  
`-config-path` is the path of all your server configs [default: `"./configs/"`]

### Example Usage

`./infrared -debug=true -config-path="."`

## Configs

Infrared handles configs similar to Nginx.
Every proxy has its own config file that has to end in `.yml` or `.yaml`.
All config options are below, but only the marked* fields are essential for a valid config file.

`DomainName`* is a [fully qualified domain name](https://en.wikipedia.org/wiki/Domain_name)  
`ListenTo` is the address that the proxy listen to for incoming connections [default: `":25565"`]  
`ProxyTo`* is the address that the proxy sends the incoming connections to  
`Timeout` is the duration before it will be shut down [default: `5m`]  

`Docker`* is a data object that represents a docker interface
- `DNSServer` is the address of the DNS that resolves container names [default: `"127.0.0.11"`]
- `ContainerName`* is the Name of the container that contains the Minecraft server
- `Portainer` is a data object that represents a Portainer interface that is only needed
if you are using [Portainer](https://www.portainer.io/) for user privilege management
  - `Address`* is the address of the Portainer instance
  - `EndpointID`* is the id of the docker endpoint
  - `Username`* is the username for the Portainer user
  - `Password`* is the password for the Portainer user

`Server` is a data object that represents a [SLP response](https://wiki.vg/Server_List_Ping)
from a vanilla Minecraft server
- `DisconnectMessage` is the text that gets displayed as reason for the disconnect
(use $username when you want to use their username) [default: `"Hey §e$username§r! The server was sleeping but it is starting now."`]  
- `Version` is the Minecraft version displayed with the placeholder [default: `"Infrared 1.15.2"`]
- `Protocol` is the [version number](https://wiki.vg/Protocol_version_numbers) of the protocol that is used [default: `578`]
- `Icon` is the path to the icon image that is displayed on the client side
- `Motd` is the Motd of a Minecraft server [default: `"Powered by Infrared"`]
- `MaxPlayers` is the maximum of players that can join the Minecraft server [default: `20`]
- `PlayersOnline` is the amount of players that are online currently on the server [default: `0`]
- `Players` is an array of players that are shown on the client side when hovered over the player count
    - `Name` is the player name displayed
    - `ID` is the UUID of the player (important for the player head next to the name)

`CallbackLog` is a data object that represents a callback log writer
- `URL` is the URL for the callback log server (logs are send in JSON via POST-Method)
- `Events` specify the logs that are sent to the callback URL (all logs are send if this is empty or nonexistent)
  - `Error` will send error logs
  - `PlayerJoin` will send player joins
  - `PlayerLeave` will send player leaves
  - `ContainerStart` will send container starts
  - `ContainerStop` will send container stops
  - `ContainerTimeout` will send container timeout start (when the last player leaves the server)

## Example Config for a Vanilla Server

`mc.example.com.yml`

```yaml
DomainName: "mc.example.com"
ProxyTo: ":8080"
Timeout: "13m37s"
Docker:
  DNSServer: "127.0.0.11"
  ContainerName: "mc"
  Portainer:
    Address: "localhost:9000"
    EndpointID: "1"
    Username: "admin"
    Password: "foobar"
Server:
  DisconnectMessage: "Sorry §e$username§r, but the server is §osleeping§r right now."
  Version: "1.14.4"
  Protocol: 498
  Icon: "/path/to/icon.png"
  Motd: "Server is currently sleeping"
  MaxPlayers: 20
  PlayersOnline: 2
  Players:
    - Name: "Steve"
      ID: "8667ba71-b85a-4004-af54-457a9734eed7"
    - Name: "Alex"
      ID: "ec561538-f3fd-461d-aff5-086b22154bce"
CallbackLog:
  URL: "http://localhost:8080/logs"
  Events:
    - "Error"
    - "PlayerJoin"
    - "PlayerLeave"
    - "ContainerStart"
```
