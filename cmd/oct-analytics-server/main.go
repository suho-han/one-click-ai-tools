// oct-analytics-server is a local-only backend for two independently exposed
// proxies: public ingestion and a tailnet-private dashboard.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/suho-han/one-click-ai-tools/internal/analyticsserver"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "oct-analytics-server:", err)
		os.Exit(1)
	}
}

func run() error {
	ingestAddr := flag.String("ingest-addr", "127.0.0.1:8843", "loopback ingestion listener for the public proxy")
	dashboardAddr := flag.String("dashboard-addr", "127.0.0.1:8844", "loopback dashboard listener for the private proxy")
	dbPath := flag.String("db", "", "SQLite file path (required)")
	flag.Parse()
	if flag.NArg() != 0 || *dbPath == "" {
		return errors.New("-db is required; unexpected positional arguments are not allowed")
	}
	if err := loopbackAddress(*ingestAddr); err != nil {
		return errors.New("ingest address must be a numeric loopback IP and port")
	}
	if err := loopbackAddress(*dashboardAddr); err != nil {
		return errors.New("dashboard address must be a numeric loopback IP and port")
	}
	svc, err := analyticsserver.Open(*dbPath)
	if err != nil {
		return err
	}
	defer svc.Close()
	ingest, err := net.Listen("tcp", *ingestAddr)
	if err != nil {
		return errors.New("cannot bind ingestion listener")
	}
	defer ingest.Close()
	dashboard, err := net.Listen("tcp", *dashboardAddr)
	if err != nil {
		return errors.New("cannot bind dashboard listener")
	}
	defer dashboard.Close()
	makeServer := func(handler http.Handler) *http.Server {
		return &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
			WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 4096,
			ErrorLog: log.New(io.Discard, "", 0)}
	}
	publicServer := makeServer(svc.IngestHandler())
	privateServer := makeServer(svc.DashboardHandler())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	failures := make(chan error, 2)
	go func() { failures <- publicServer.Serve(ingest) }()
	go func() { failures <- privateServer.Serve(dashboard) }()
	pruneDone := make(chan struct{})
	go func() {
		defer close(pruneDone)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if svc.Prune() != nil {
					failures <- errors.New("retention prune failed")
					return
				}
			}
		}
	}()
	select {
	case <-ctx.Done():
	case err = <-failures:
		if err == http.ErrServerClosed {
			err = nil
		}
	}
	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = publicServer.Shutdown(shutdownCtx)
	_ = privateServer.Shutdown(shutdownCtx)
	<-pruneDone
	if err != nil {
		return errors.New("analytics listener or retention prune stopped unexpectedly")
	}
	return nil
}

func loopbackAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	number, parseErr := strconv.Atoi(port)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() || parseErr != nil || number < 1 || number > 65535 {
		return errors.New("not a loopback address")
	}
	return nil
}
