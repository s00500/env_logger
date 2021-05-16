# env_logger

This is a super simple project which aims to help out with setting up logging correctly in your project. It is a true drop in replacement for `logrus` logger atm.

# Usage

The entire logging framework is configured via a single environment variable `LOG`. The variable is a comma delimited list
of packages and their respective log-levels. (falling back to InfoLevel if not configured).

## Bonus tricks

Some bonus modifiers exist for the log config: 
- **ln** enables printing of line numbers
- **gr** adds number of goroutines to each log statement
- **grl** adds number of goroutines to each log statement and starts a loop printing the number of routines every second


## Bonus functions

- **log.Must**, **log.MustFatal** if the passed error is not nil log it and throw a panic or end the program
- **log.Should**, **log.ShouldWarn** if the passed error is not nul just log it, returns true if error has been printed
- **log.Wrap** can be used with Should and must functions to provide additional error information (eg: log.Should(log.Wrap(err, "on testing %s", somedata)))
- **log.Indent** can be used to prety print the public fields of a structure (eg: log.Info(log.Indent(myStructure)))

## Examples

``` shell
LOG=foo=debug,bar=warn go run
```

This configures the `foo` package at loglevel _Debug_, the bar package at loglevel _Warn_ and the default/fallback logger at Info.

``` shell
LOG=foo=info,debug,bar=warn go run
```

This is the same as the previous example, except foo is now at loglevel _Info_, and the default loglevel is _Debug_.

``` shell
LOG=debug go run
```

This example sets everything to _Debug_.
