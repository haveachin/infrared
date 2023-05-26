# Java Config

Everything that lives in the `java` object is considerd to be part of the Java Config.

```yml{2}
# This is an example of the java object:
java:
  gateways:
    ...
  servers:
    ...
```

## Gateways

### What is a gateway?

- It has a ***unique*** name.
- It can hold one or more listeners.

### What is a listener?

- It has a ***unique*** name in the scope of this gateway.
- It binds to a ***unique*** [ip]:port combination.
- [It can receive Proxy Protocol v2.](/guide/proxy-protocol)
- [It can receive RealIP.](/guide/real-ip)
- It has error messages if it cannot relay to player to a server.

### Examples

Basic example
```yml{5,11}
java:
  gateways:
    # alice is the unique name of this gateway.
    #
    alice:
      # Now we create some listeners for this gateway here.
      #
      listeners:
        # bob is the unique name of this listener.
        #
        bob:
          bind: :12345
```

Advanced example:
```yml
java:
  gateways:
    default:
      listeners:
        default:
          bind: :25565

          # The message that is displayed to a client when they try to connect via an invalid domain.
          #
          serverNotFoundMessage: Sorry {{username}}, but {{requestedAddress}} was not found.

          serverNotFoundStatus:
            versionName: Infrared

            # The protocol number. For more info see https://wiki.vg/Protocol_version_numbers
            #
            protocolNumber: 0
            maxPlayerCount: 0
            playerCount: 0
            iconPath: icons/default.png
            motd: |
              Powered by Infrared
              §c{{requestedAddress}} was not found.
```

## Servers

## Advanced