package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"GoRig/internal/base"
	"GoRig/internal/console"
)

func main() {
	diagnostics, err := console.Setup(os.Args[1:])
	if err != nil {
		console.Error(false, err.Error())
		os.Exit(1)
	}
	fail := func(text string, err error) { console.Error(diagnostics, text+err.Error()); os.Exit(1) }
	flag.Bool("console", false, "показывать консоль диагностики")
	exe, err := os.Executable()
	if err != nil {
		fail("Не удалось определить каталог программы: ", err)
	}
	path := flag.String("db", filepath.Join(filepath.Dir(exe), "gorig-base.sqlite"), "путь к файлу базы данных")
	port := flag.String("port", "8765", "локальный порт HTTP")
	flag.Parse()
	store, err := base.Open(*path)
	if err != nil {
		fail("Не удалось открыть базу: ", err)
	}
	defer store.Close()
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", *port))
	if err != nil {
		fail("Не удалось открыть порт: ", err)
	}
	address := "http://" + listener.Addr().String() + "/"
	fmt.Printf("GoRig Base\nБаза: %s\nОткройте в браузере: %s\nДля остановки используйте меню значка в трее или Ctrl+C.\n", *path, address)
	stop := make(chan struct{}, 1)
	requestShutdown := func() {
		select {
		case stop <- struct{}{}:
		default:
		}
	}
	server := &http.Server{Handler: base.HandlerWithShutdown(store, requestShutdown), ReadHeaderTimeout: 10 * time.Second}
	if err := runServer(server, listener, address, stop, store.Organizations); err != nil {
		console.Error(diagnostics, "Ошибка работы программы: "+err.Error())
	}
}
