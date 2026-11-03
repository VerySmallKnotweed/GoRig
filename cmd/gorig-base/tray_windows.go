//go:build windows

package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"GoRig/internal/base"
	"GoRig/internal/icons"

	"github.com/gogpu/systray"
)

func runServer(server *http.Server, listener net.Listener, address string, stop <-chan struct{}, organizations func() ([]base.Organization, error)) error {
	// The Win32 window and its message loop must use the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	tray := systray.New()
	var stopTray sync.Once
	removeTray := func() { stopTray.Do(tray.Remove) }
	openURL := func(url string) {
		// Shell handler opens the URL in the user's default browser.
		cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
		if err := cmd.Start(); err != nil {
			log.Printf("Не удалось открыть браузер: %v. Адрес: %s", err, url)
		} else {
			go func() { _ = cmd.Wait() }()
		}
	}
	refreshMenu := func() {
		menu := systray.NewMenu()
		orgs, err := organizations()
		if err != nil {
			log.Printf("Не удалось загрузить организации для меню трея: %v", err)
		} else {
			for _, entry := range organizationMenuEntries(address, orgs) {
				url := entry.URL
				menu.Add(entry.Label, func() { openURL(url) })
			}
			menu.AddSeparator()
		}
		menu.Add("Выход", removeTray)
		tray.SetMenu(menu)
	}
	tray.SetIcon(icons.BasePNG).SetTooltip("GoRig Base")
	refreshMenu()
	tray.OnRightClick(refreshMenu)
	tray.OnClick(func() { openURL(address) })
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
