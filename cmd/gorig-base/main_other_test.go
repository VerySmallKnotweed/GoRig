//go:build !windows

package main

import (
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"GoRig/internal/base"
)

func TestWebShutdownStopsServer(t *testing.T) {
	store, err := base.Open(filepath.Join(t.TempDir(), "base.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	stop := make(chan struct{}, 1)
	server := &http.Server{Handler: base.HandlerWithShutdown(store, func() {
		select {
		case stop <- struct{}{}:
		default:
		}
	})}
	done := make(chan error, 1)
	go func() { done <- runServer(server, listener, "", stop) }()
	url := "http://127.0.0.1:" + strconv.Itoa(listener.Addr().(*net.TCPAddr).Port) + "/api/shutdown"
	response, err := http.Post(url, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status %d", response.StatusCode)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}
