//go:build windows

package main

import (
	"context"
	_ "embed"
	"errors"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/gogpu/systray"
)

//go:embed assets/tray.png
var trayIcon []byte

func runServer(server *http.Server, listener net.Listener, address string, stop <-chan struct{}) error {
	// The Win32 window and its message loop must use the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	tray := systray.New()
	var stopTray sync.Once
	removeTray := func() { stopTray.Do(tray.Remove) }
	menu := systray.NewMenu()
	menu.Add("Выход", removeTray)
	tray.SetIcon(trayIcon).SetTooltip("GoRig Base").SetMenu(menu)
	tray.OnClick(func() {
		// Shell handler opens the URL in the user's default browser.
		cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", address)
		if err := cmd.Start(); err != nil {
			log.Printf("Не удалось открыть браузер: %v. Адрес: %s", err, address)
		} else {
			go func() { _ = cmd.Wait() }()
		}
	})
	tray.Show()
	serveResult := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		serveResult <- err
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			removeTray()
		}
	}()
	finished := make(chan struct{})
	go func() {
		select {
		case <-stop:
			removeTray()
		case <-finished:
		}
	}()
	trayErr := tray.Run()
	close(finished)
	// Once the tray closes, stop the HTTP server before the DB is closed.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		_ = server.Close()
	}
	serveErr := <-serveResult
	if trayErr != nil {
		return trayErr
	}
	if !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	return nil
}
