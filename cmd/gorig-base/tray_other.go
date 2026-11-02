//go:build !windows

package main

import (
	"net"
	"net/http"
)

func runServer(server *http.Server, listener net.Listener, _ string) error {
	return server.Serve(listener)
}
