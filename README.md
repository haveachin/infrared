# Infrared

An ultra lightweight minecraft reverse proxy and idle placeholder:
Ever wanted to have only one exposed port at your server for multiple minecraft servers? Then infrared is the tool you need! Infrared works as a reverse proxy using a subdomains to connect clients to a specific minecraft server.

## Features

- [ ] Reverse Proxy
- [ ] Display Placeholder Server
- [ ] Autostart Server when pinged
- [ ] gRPC API

## Installation

### Build it yourself

- Download the latest release of this repository.
- You need Go.  
Download it [here](https://golang.org/dl/) or with your favorite packet manager.

After that **navigate in the project folder** and pull all dependecies:

```bash
go get -u
```

Now you can just build it:

```bash
go build ./cmd/infrared
```

or istall it:

```bash
go install ./cmd/infrared
```

### Download a build

Just download a build from [here](https://github.com/haveachin/infrared/releases) and your good to go.

## Example Config

```yaml
DomainName: "mc.example.com"
ListenTo: ":25565"
ProxyTo: ":8080"
Deadline: "5s"
PingCommand: "java -Xmx512M -Xms512M -jar minecraft_server.jar nogui"
Placeholder:
    Motd: "Server is currently sleeping"
    MaxPlayers: 20
    PlayersOnline: 0
```

`DomainName` is a [fully qualified domain name](https://en.wikipedia.org/wiki/Domain_name)  
`ListenTo` is the address that the proxy listen to for incoming connections  
`ProxyTo` is the address that the proxy sents the incoming connections to  
`Deadline` is the duration that a connection can idle for without sending any data  
`PingCommand` is a command that is executed when the server is pinged while being offline  
`Placeholder` is a data object that represents a ping response from a vannila minecraft server  
`Motd` is the Motd of a minecraft server  
`MaxPlayers` is the maximum of players that can join the minecraft server  
`PlayersOnline` is the amount of players that are online currently on the server  
