<p align="center">
   <img width="300" height="auto" src="https://i.imgur.com/sD8cjJc.png">
 </p>

# Infrared

An ultra lightweight Minecraft reverse proxy and idle placeholder:
Ever wanted to have only one exposed port on your server for multiple Minecraft servers?
Then Infrared is the tool you need!
Infrared works as a reverse proxy using a subdomain to connect clients to a specific Minecraft server.
It works similar to Nginx for those of you who are familiar. 

## Features

- [x] Reverse Proxy
- [x] Display Placeholder Server
- [x] Autostart Server when pinged
- [x] Logger Callback URLs
- [ ] REST API

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

`INFRARED_CONFIG_PATH` is the path to all your server configs [default: `"./configs/"`]

## Command-Line Flags

`-config-path` specifies the path to all your server configs [default: `"./configs/"`]

### Example Usage

`./infrared -config-path="."`

## Proxy Config

| Field Name        | Type    | Required | Default                                        | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
|-------------------|---------|----------|------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| domainName        | String  | true     | localhost                                      | Should be [fully qualified domain name](https://en.wikipedia.org/wiki/Domain_name). <br>Note: Every string is accepted. So `localhost` is also valid.                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| listenTo          | String  | true     | :25565                                         | The address (usually just the port; so short term `:port`) that the proxy should listen to for incoming connections.<br>Accepts basically every address format you throw at it. Valid examples: `:25565`, `localhost:25565`, `0.0.0.0:25565`, `127.0.0.1:25565`, `example.de:25565`                                                                                                                                                                                                                                                                                                        |
| proxyTo           | String  | true     |                                                | The address that the proxy should send incoming connections to. Accepts Same formats as the `listenTo` field.                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| disconnectMessage | String  | false    | Sorry {{username}}, but the server is offline. | The message a client sees when he gets disconnected from Infrared due to the server on `proxyTo` won't respond. Currently available placeholders:<br>- `username` the username of player that tries to connect<br>- `now` the current server time<br>- `remoteAddress` the address of the client that tries to connect<br>- `localAddress` the local address of the server<br>- `domain` the domain of the proxy (same as `domainName`)<br>- `proxyTo` the address that the proxy proxies to (same as `proxyTo`)<br>- `listenTo` the address that Infrared listens on (same as `listenTo`) |
| time           | Integer | true     | 1000                                           | The time in milliseconds for the proxy to wait for a ping response before the host (the address you proxyTo) will be declared as offline. This "online check" will be resend for every new connection.                                                                                                                                                                                                                                                                                                                                                                                     |
| proxyProtocol     | Boolean | false    | false                                          | If Infrared should use HAProxy's Proxy Protocol for IP **forwarding**.<br>Warning: You should only ever set this to true if you now that the server you `proxyTo` is compatible.                                                                                                                                                                                                                                                                                                                                                                                                           |
| docker            | Object  | false    | See [Docker](#Docker)                          | Optional Docker configuration to automatically start a container and stop it again if unused.  <br>Note: Infrared will not take direct connections into account. Be sure to route all traffic that connects to the container through Infrared.                                                                                                                                                                                                                                                                                                                                             |
| onlineStatus      | Object  | false    |                                                | This is the response that Infrared will give when a client asks for the server status and the server is online.                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| offlineStatus     | Object  | false    | See [Response Status](#Response Status)        | This is the response that Infrared will give when a client asks for the server status and the server is offline.                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| callbackServer    | Object  | false    | See [Callback Server](#Callback Server)        | Optional callback server configuration to send events as a POST request to a specified URL.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |

### Docker

| Field Name    | Type   | Required | Default    | Description                                                                 |
|---------------|--------|----------|------------|-----------------------------------------------------------------------------|
| dnsServer     | String | false    | 127.0.0.11 | The address of the DNS that resolves the container names.                   |
| containerName | String | true     |            | The name of the container that should be automatically started/stopped.     |
| portainer     | Object | false    |            | Optional [Portainer](#Portainer) configuration for authorization management.|

#### Portainer

More info on [Portainer](https://www.portainer.io/).

| Field Name | Type   | Required | Default | Description                                                                   |
|------------|--------|----------|---------|-------------------------------------------------------------------------------|
| address    | String | true     |         | URL of the Portainer instance.                                                |
| endpointId | String | true     |         | The ID typically an integer of the docker endpoint in the portainer instance. |
| username   | String | true     |         | Username for the Portainer user.                                              |
| password   | String | true     |         | Password for the Portainer user.                                              |

### Response Status

| Field Name     | Type    | Required | Default         | Description                                                                                                                                          |
|----------------|---------|----------|-----------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| versionName    | String  | false    | Infrared 1.16.5 | The version name of the Minecraft Server.                                                                                                            |
| protocolNumber | Integer | true     | 754             | The protocol version number.                                                                                                                         |
| maxPlayers     | Integer | false    | 20              | The maximum number of players that can join the server.<br>Note: Infrared will not limit more players from joining. This number is just for display. |
| playersOnline  | Integer | false    | 0               | The number of online players.<br>Note: Infrared will not that this number is also just for display.                                                  |
| playerSamples  | Array   | false    |                 | An array of player samples. See [Player Sample](#Player Sample).                                                                                     |
| iconPath       | String  | false    |                 | The path to the server icon.                                                                                                                         |
| motd           | String  | false    |                 | The motto of the day, short MOTD.                                                                                                                    |

#### Player Sample

| Field Name | Type   | Required | Default | Description             |
|------------|--------|----------|---------|-------------------------|
| Name       | String | true     |         | Username of the player. |
| uuid       | String | false    |         | UUID of the player.     |

### Callback Server

| Field Name | Type   | Required | Default | Description                                                                                                                                                                                                                                                                             |
|------------|--------|----------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| url        | String | true     |         | URL of the callback server URL.                                                                                                                                                                                                                                                         |
| events     | Array  | true     |         | A string array of event names. Currently available event names are:<br>- `Error` will send error logs<br>- `PlayerJoin` will send player joins<br>- `PlayerLeave` will send player leaves<br>- `ContainerStart` will send container starts<br>- `ContainerStop` will send container stops |


### Examples

#### Minimal Config

<details>
<summary>min.example.com</summary>

```json
{
  "proxyTo": ":8080"
}
```

</details>

#### Full Config

<details>
<summary>full.example.com</summary>

```json
{
  "domainName": "mc.example.com",
  "listenTo": ":25565",
  "proxyTo": ":8080",
  "time": 1000,
  "disconnectMessage": "Username: {{username}}\nNow: {{now}}\nRemoteAddress: {{remoteAddress}}\nLocalAddress: {{localAddress}}\nDomain: {{domain}}\nProxyTo: {{proxyTo}}\nListenTo: {{listenTo}}",
  "docker": {
    "dnsServer": "127.0.0.11",
    "containerName": "mc",
    "time": 30000,
    "portainer": {
      "address": "localhost:9000",
      "endpointId": "1",
      "username": "admin",
      "password": "foobar"
    }
  },
  "onlineStatus": {
    "versionName": "1.16.5",
    "protocolNumber": 754,
    "maxPlayers": 20,
    "playersOnline": 2,
    "playerSamples": [
      {
        "name": "Steve",
        "uuid": "8667ba71-b85a-4004-af54-457a9734eed7"
      },
      {
        "name": "Alex",
        "uuid": "ec561538-f3fd-461d-aff5-086b22154bce"
      }
    ],
    "motd": "Join us!"
  },
  "offlineStatus": {
    "versionName": "1.16.5",
    "protocolNumber": 754,
    "maxPlayers": 20,
    "playersOnline": 0,
    "motd": "Server is currently offline"
  },
  "callbackServer": {
    "url": "https://mc.example.com/callback",
    "events": [
      "Error",
      "PlayerJoin",
      "PlayerLeave",
      "ContainerStart",
      "ContainerStop"
    ]
  }
}
```

</details>
