# Infrared

An ultra lightweight minecraft reverse proxy and idle placeholder:
Ever wanted to have only one exposed port at your server for multiple minecraft servers? Then infrared is the tool you need! Infrared works as a reverse proxy using a subdomains to connect clients to a specific minecraft server.

## Features

- [ ] Reverse Proxy
- [ ] Display placeholder server
- [ ] gRPC API

## Example Config

```yaml
DomainName: "mc.example.com"
ListenTo: ":25565"
ProxyTo: ":8080"
Deadline: "5s"
Placeholder:
    Motd: "Server is currently sleeping"
    MaxPlayers: 20
    PlayersOnline: 0
```

`DomainName` is a [fully qualified domain name](https://en.wikipedia.org/wiki/Domain_name)  
`ListenTo` is the address that the proxy listen to for incoming connections  
`ProxyTo` is the address that the proxy sents the incoming connections to  
`Deadline` is the duration that a connection can idle for without sending any data  
`Placeholder` is a data object that represents a ping response from a vannila minecraft server  
`Motd` is the Motd of a minecraft server  
`MaxPlayers` is the maximum of players that can join the minecraft server  
`PlayersOnline` is the amount of players that are online currently on the server  
