# Rate Limit IPs

You can rate limit by IP address using the `rateLimit` filter.
This can be easily activated in your [**global config**](../config/index) by adding this:

```yml{2-16}
filters:
  # Rate Limiter will only allow an IP address to connect a specified
  # amount of times in a given time frame.
  #
  rateLimiter:
    # Request Limit is the amount of times an IP address can create
    # a new connection before it gets blocked.
    #
    requestLimit: 10
    
    # Windows Length is the time frame for the Request Limit.
    #
    windowLength: 1s
```
