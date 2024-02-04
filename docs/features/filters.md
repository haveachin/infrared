# Filters

Filter are hooks that trigger befor a connection is processed.
They are used as preconditions to validate a connection.

## Use Filters

To use filters you just need to a this to your [**global config**](../config/index.md):

```yml
# Filter are hooks that trigger befor a connection is processed.
# They are used as preconditions to validate a connection.
#
filters:
```

Now you actually need to add filters to your config.
This is a list of all the filters that currently exist:

- [Rate Limiter](rate-limiter)