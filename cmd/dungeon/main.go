// Command dungeon serves the Dungeon cockpit on localhost.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dungeon/internal/server"
)

// Stamped at build time with -ldflags -X.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "dungeon:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("missing command")
	}
	switch args[0] {
	case "serve":
		return serve(args[1:])
	case "version":
		fmt.Printf("dungeon %s (commit %s, built %s)\n", version, commit, date)
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: dungeon <command>

commands:
  serve     run the local server (loopback only)
  version   print version information
`)
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:7878", "loopback address to listen on (port 0 picks a free one)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	token, err := server.NewToken()
	if err != nil {
		return err
	}
	cfg, err := buildConfig(context.Background(), *addr, token)
	if err != nil {
		return err
	}
	if cfg.Claude == "" {
		fmt.Fprintln(os.Stderr, "warning:", cfg.ClaudeErr)
	}
	if cfg.Party.Defaulted {
		fmt.Fprintln(os.Stderr, "note: no party.toml found; using the built-in heroes")
	}
	srv, err := server.New(cfg)
	if err != nil {
		return err
	}
	ln, err := srv.Listen()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("dungeon %s listening on http://%s/?token=%s\n", version, ln.Addr(), token)
	fmt.Println("open that URL once; the token is per launch and is exchanged for a session cookie.")

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
