package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Mhmdabbasi/bt-manager/internal/app"
	"github.com/Mhmdabbasi/bt-manager/internal/config"
	"github.com/Mhmdabbasi/bt-manager/internal/httpapi"
	"github.com/Mhmdabbasi/bt-manager/internal/store"
)

func main() {
	dataDirDefault, err := config.DefaultDataDir()
	if err != nil { fatal("resolve data directory", err) }
	listen := flag.String("listen", "127.0.0.1:8080", "local HTTP listen address")
	dataDir := flag.String("data-dir", dataDirDefault, "local application data directory")
	flag.Parse()
	if err := validateLoopback(*listen); err != nil { fatal("unsafe listen address", err) }
	if err := os.MkdirAll(*dataDir, 0700); err != nil { fatal("create data directory", err) }
	db, err := store.Open(filepath.Join(*dataDir, "bt-manager.db"))
	if err != nil { fatal("open database", err) }
	defer db.Close()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	srv := &http.Server{Addr:*listen, Handler:httpapi.New(app.New(db, logger), logger), ReadHeaderTimeout:5*time.Second, ReadTimeout:15*time.Second, WriteTimeout:30*time.Second, IdleTimeout:60*time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func(){ <-ctx.Done(); shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second); defer cancel(); _ = srv.Shutdown(shutdown) }()
	logger.Info("bt-manager started", "address", *listen, "dataDir", *dataDir)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) { fatal("HTTP server", err) }
}

func validateLoopback(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil { return err }
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil || !ip.IsLoopback() { return errors.New("LAN binding is disabled until authentication and HTTPS are implemented") }
	return nil
}
func fatal(message string, err error) { _, _ = os.Stderr.WriteString("bt-manager: "+message+": "+err.Error()+"\n"); os.Exit(1) }
