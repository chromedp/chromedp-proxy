# About chromedp-proxy

`chromedp-proxy` is a simple command-line tool to proxy and log [Chrome
DevTools Protocol][devtools-protocol] sessions sent from a CDP client to a CDP
browser session. `chromedp-proxy` captures and (by default) logs all of the
WebSocket messages sent during a CDP session between a remote and local
endpoint, and can be used to expose a CDP browser listening on localhost to a
remote endpoint.

`chromedp-proxy` is mainly used to capture and debug the wireline protocol sent
from DevTools/Selenium/Puppeteer to Chrome/Chromium/headless_shell/etc. It was
originally written for debugging wireline problems/issues with the
[`chromedp`][chromedp] project.

## Installing

Install in the usual Go way:

```sh
$ go get -u github.com/chromedp/chromedp-proxy
```

## Using

By default, `chromedp-proxy` listens on `localhost:9223` and proxies
requests to/from `localhost:9222`:

```sh
$ chromedp-proxy
```

`chromedp-proxy` can also be used to expose a local Chrome instance on an
external address/port:

```sh
$ chromedp-proxy -l 192.168.1.10:9222
```

By default, `chromedp-proxy` logs to both `stdout` and to
`$PWD/logs/cdp-<id>.log`, but that can be changed through flags:

```sh
# only log to stdout
$ chromedp-proxy -n

# or only log to stdout by specifying an empty log name
$ chromedp-proxy -log ''

# log to /var/log/cdp/session-<id>.log
$ chromedp-proxy -log '/var/log/cdp/session-%s.log'
```

### Command-line options

```sh
$ ./chromedp-proxy -help
Usage of ./chromedp-proxy:
  -l string
    	listen address (default "localhost:9223")
  -log string
    	log file mask (default "logs/cdp-%s.log")
  -n	disable logging to file
  -r string
    	remote address (default "localhost:9222")
```

[devtools-protocol]: https://chromedevtools.github.io/devtools-protocol/
[chromedp]: https://github.com/chromedp
