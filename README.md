# Infrared

An ultra lightweight minecraft reverse proxy and idle placeholder:
Ever wanted to have only one exposed port at your server for multiple minecraft servers? Then infrared is the tool you need! Infrared works as a reverse proxy using a subdomains to connect clients to a specific minecraft server.

## Features

- [x] Reverse Proxy
- [x] Display Placeholder Server
- [x] Autostart Server when pinged
- [ ] API for logging via InfluxDB
- [ ] gRPC API for live data

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

or install it:

```bash
go install ./cmd/infrared
```

### Download a build

Just download a build from [here](https://github.com/haveachin/infrared/releases) and your good to go.

## Environment Variables

**!!Flags override environment variables!!**  
`INFRARED_ADDRESS` is the address that the proxy listens to  
`INFRARED_CONFIG_PATH` is the path of all your server configs

## Flags

**!!Flags override environment variables!!**  
`--address` is the address that the proxy listens to [default: ":25565"]  
`--config_path` is the path of all your server configs [default: "./configs/"]

### Example Usage

`./infrared --address ":8080" --config_path "."`

## Example Config for a Vanilla Server

```yaml
DomainName: "mc.example.com"
ListenTo: ":25565"
ProxyTo: ":8080"
DisconnectMessage: "Sorry §e$username§r, but the server is §osleeping§r right now."
Timeout: "13m37s"
Command: "java -server -Xmx512M -jar minecraft_server.jar nogui"
Docker:
    ContainerID: "4c01db0b339c"
    Portainer:
        Address: "localhost:9000"
        EndpointID: "1"
        Username: "admin"
        Password: "foobar"
Placeholder:
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
```

`DomainName` is a [fully qualified domain name](https://en.wikipedia.org/wiki/Domain_name)  
`ListenTo` is the address that the proxy listen to for incoming connections **not implemented yet**  
`ProxyTo` is the address that the proxy sents the incoming connections to  
`DisconnectMessage` is the text that gets diplayed as reason for the disconnect (use $username when you want to use their username)  
`Timeout` is the duration befor it will be shut down  
`Command` is the command that start the minecraft server  

**Only used if the** `Command` **is not present**  
`Docker` is a data object that represents a docker interface.

- `ContainerID` is the ID of the container that contains the minecraft server  
**Only needed if you are using [Portainer](https://www.portainer.io/) for user privilege management**
- `Portainer` is a data object that represents a portainer interface
  - `Address` is the address of the portainer instance
  - `EndpointID` is the id of the docker endpoint
  - `Username` is the username for the portainer user
  - `Password` is the password for the portainer user

`Placeholder` is a data object that represents a [SLP response](https://wiki.vg/Server_List_Ping) from a vannila minecraft server

- `Version` is the minecraft version diplayed with the placeholder
- `Protocol` is the [version number](https://wiki.vg/Protocol_version_numbers) of the protocol that is used
- `Icon` is the path to the icon image that is diplayed on the client side
- `Motd` is the Motd of a minecraft server
- `MaxPlayers` is the maximum of players that can join the minecraft server
- `PlayersOnline` is the amount of players that are online currently on the server
- `Players` is an array of players that are shown on the client side when hovered over the player count
- `Name` is the player name displayed
- `ID` is the UUID of the player (important for the player head that is displayed next to the name)
