package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"tape-preservation-incident-api/internal/app"
	"tape-preservation-incident-api/internal/httpapi"
	"tape-preservation-incident-api/internal/store"
)

type runtime struct {
	server   *http.Server
	listener net.Listener
}

func buildRuntime(cfg config) (*runtime, error) {
	repository, err := store.Open(cfg.dataDir)
	if err != nil {
		return nil, err
	}
	service := app.NewService(repository, app.UTCClock{})
	api := httpapi.New(service)
	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return nil, fmt.Errorf("监听 %s: %w", cfg.addr, err)
	}
	server := &http.Server{
		Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10,
	}
	return &runtime{server: server, listener: listener}, nil
}

func (rt *runtime) serve(errorChannel chan<- error) {
	err := rt.server.Serve(rt.listener)
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	errorChannel <- err
}

func (rt *runtime) shutdown(ctx context.Context) error {
	return rt.server.Shutdown(ctx)
}
