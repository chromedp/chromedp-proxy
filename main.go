// chromedp-proxy provides a cli utility that will proxy requests from a Chrome
// DevTools Protocol client to a browser instance.
//
// chromedp-proxy is particularly useful for recording events/data from
// Selenium (ChromeDriver), Chrome DevTools in the browser, or for debugging
// remote application instances compatible with the devtools protocol.
//
// Please see README.md for more information on using chromedp-proxy.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/gorilla/websocket"
)

func main() {
	listen := flag.String("l", "localhost:9223", "listen address")
	remote := flag.String("r", "localhost:9222", "remote address")
	noLog := flag.Bool("n", false, "disable logging to file")
	logMask := flag.String("log", "logs/cdp-%s.log", "log file mask")
	flag.Parse()
	if err := run(context.Background(), *listen, *remote, *noLog, *logMask); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, listen, remote string, noLog bool, logMask string) error {
	mux := http.NewServeMux()
	simplep := httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   remote,
	})
	mux.Handle("/json", simplep)
	mux.Handle("/", simplep)
	mux.HandleFunc("/devtools/", func(res http.ResponseWriter, req *http.Request) {
		id := path.Base(req.URL.Path)
		f, logger := createLog(noLog, logMask, id)
		if f != nil {
			defer f.Close()
		}
		logger.Printf("---------- connection from %s ----------", req.RemoteAddr)
		ver, err := checkVersion(ctx, remote)
		if err != nil {
			msg := fmt.Sprintf("version error, got: %v", err)
			logger.Println(msg)
			http.Error(res, msg, http.StatusInternalServerError)
			return
		}
		logger.Printf("endpoint %s reported: %s", remote, string(ver))
		endpoint := "ws://" + remote + path.Join(path.Dir(req.URL.Path), id)
		// connect outgoing websocket
		logger.Printf("connecting to %s", endpoint)
		out, pres, err := wsDialer.Dial(endpoint, nil)
		if err != nil {
			msg := fmt.Sprintf("could not connect to %s, got: %v", endpoint, err)
			logger.Println(msg)
			http.Error(res, msg, http.StatusInternalServerError)
			return
		}
		defer pres.Body.Close()
		defer out.Close()
		logger.Printf("connected to %s", endpoint)
		// connect incoming websocket
		logger.Printf("upgrading connection on %s", req.RemoteAddr)
		in, err := wsUpgrader.Upgrade(res, req, nil)
		if err != nil {
			msg := fmt.Sprintf("could not upgrade websocket from %s, got: %v", req.RemoteAddr, err)
			logger.Println(msg)
			http.Error(res, msg, http.StatusInternalServerError)
			return
		}
		defer in.Close()
		logger.Printf("upgraded connection on %s", req.RemoteAddr)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		errc := make(chan error, 1)
		go proxyWS(ctx, logger, "<-", in, out, errc)
		go proxyWS(ctx, logger, "->", out, in, errc)
		<-errc
		logger.Printf("---------- closing %s ----------", req.RemoteAddr)
	})
	return http.ListenAndServe(listen, mux)
}

const (
	incomingBufferSize = 10 * 1024 * 1024
	outgoingBufferSize = 25 * 1024 * 1024
)

var wsUpgrader = &websocket.Upgrader{
	ReadBufferSize:  incomingBufferSize,
	WriteBufferSize: outgoingBufferSize,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var wsDialer = &websocket.Dialer{
	ReadBufferSize:  outgoingBufferSize,
	WriteBufferSize: incomingBufferSize,
}

// proxyWS proxies in and out messages for a websocket connection, logging the
// message to the logger with the passed prefix. Any error encountered will be
// sent to errc.
func proxyWS(ctx context.Context, logger *log.Logger, prefix string, in, out *websocket.Conn, errc chan error) {
	var mt int
	var buf []byte
	var err error
	for {
		select {
		default:
			mt, buf, err = in.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			logger.Println(prefix, string(buf))
			err = out.WriteMessage(mt, buf)
			if err != nil {
				errc <- err
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// checkVersion retrieves the version information for the remote endpoint, and
// formats it appropriately.
func checkVersion(ctx context.Context, remote string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://"+remote+"/json/version", nil)
	if err != nil {
		return nil, err
	}
	cl := &http.Client{}
	res, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var v map[string]string
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("expected json result: %w", err)
	}
	return body, nil
}

// createLog creates a log for the specified id based on flags.
func createLog(noLog bool, logMask, id string) (io.Closer, *log.Logger) {
	var f io.Closer
	var w io.Writer = os.Stdout
	if !noLog && logMask != "" {
		filename := logMask
		if strings.Contains(logMask, "%s") {
			filename = fmt.Sprintf(logMask, cleanRE.ReplaceAllString(id, ""))
		}
		l, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			panic(err)
		}
		f, w = l, io.MultiWriter(os.Stdout, l)
	}
	return f, log.New(w, "", log.LstdFlags)
}

var cleanRE = regexp.MustCompile(`[^a-zA-Z0-9_\-\.]`)
