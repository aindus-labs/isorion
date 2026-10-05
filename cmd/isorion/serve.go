package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/isorion-lang/isorion/pkg/isorion"
)

// cmdServe exposes the library over HTTP — level 3 of the interface
// contract. `isorion serve [-addr :8080]` mounts pkg/isorion.Handler.
func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", ":8080", "listen address")
	flagArgs, posArgs := splitArgs(args)
	if err := fs.Parse(append(flagArgs, posArgs...)); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("serve takes no arguments")
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           isorion.Handler(version, commit, date),
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Printf("isorion %s serving on http://%s (endpoints: /v1/compile /v1/import /v1/validate /v1/targets /v1/importers /v1/version)\n",
		version, ln.Addr())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() { // graceful shutdown: Ctrl-C / SIGTERM → 5s drain
		<-ctx.Done()
		dctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(dctx)
	}()
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
