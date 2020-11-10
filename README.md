# env_logger

This is a super simple project which aims to help out with setting up logging correctly in your project. It is a true drop in replacement for `logrus` logger atm.

Currently it only supports `logrus`, but PR's are welcome to support additional loggers.

# Usage

The entire logging framework is configured via a single environment variable `GOLANG_LOG`. The variable is a comma delimited list
of packages and their respective log-levels. (falling back to InfoLevel if not configured).

## Examples

``` shell
GOLANG_LOG=foo=debug,bar=warn go run
```

This configures the `foo` package at loglevel _Debug_, the bar package at loglevel _Warn_ and the default/fallback logger at Info.

``` shell
GOLANG_LOG=foo=info,debug,bar=warn go run
```

This is the same as the previous example, except foo is now at loglevel _Info_, and the default loglevel is _Debug_.

``` shell
GOLANG_LOG=debug go run
```

This example sets everything to _Debug_.


# TODO

- Add all log functions from logerus
- Add module name to log statement (at least the final part of the path)
- add an interface so that any logger can be injected as the canonical logger (currently only logrus is supported)

``` go
type Logger struct {
  New() -> Logger // used to instantiate a new logger
  Debug() // emit debug message
  Info()
  Warn()
  Panic()
}
```
