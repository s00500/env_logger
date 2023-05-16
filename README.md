# env_logger

This is a super simple project which aims to help out with setting up logging correctly in your project. It is a true drop in replacement for `logrus` logger atm.

# Usage

The entire logging framework is configured via a single environment variable `LOG`. The variable is a comma delimited list
of packages and their respective log-levels. (falling back to InfoLevel if not configured).

# Windows support

This logger should be fully able to work colored on windows! TTY detection may fail though, so to ensure that it does not set the environment variable *CLICOLOR_FORCE=1* in your shell.
## Bonus tricks

Some bonus modifiers exist for the log config: 
- **ln** enables printing of line numbers
- **gr** adds number of goroutines to each log statement
- **grl** adds number of goroutines to each log statement and starts a loop printing the number of routines every second
- **pp** enables pprof and dynamic log config via http requests on 11111, port can be changed with ppport=<port> (all of this requires the package to be built with -tags logpprof). The endpoint for the logconfig is POST /logstring. Send the new logstring as body
- **mut=10** allows to set runtime.SetMutexProfileFraction(val)
- **blk=10** allows to set runtime.SetBlockProfileFraction(val)

## Bonus functions

- **log.Must**, **log.MustFatal** if the passed error is not nil log it and throw a panic or end the program
- **log.Should**, **log.ShouldWarn** if the passed error is not nul just log it, returns true if error has been printed
- **log.Wrap** can be used with Should and must functions to provide additional error information (eg: log.Should(log.Wrap(err, "on testing %s", somedata)))
- **log.ShouldWrap** convenience for the above
- **log.Indent** can be used to prety print the public fields of a structure (eg: log.Info(log.Indent(myStructure)))
- **log.Timer and log.TimerEnd** can be used to quickly measure the time between 2 places with a key, similar to js. this does not log on its own, use with one of the standard log functions (just like .Indent above)

## Dynamic log config
If pp is active and tags logpprof have been set use this command to change the logconfig dynamically

`curl -X POST -d 'grl' http://localhost:11111/logstring`

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
