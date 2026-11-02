package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"GoRig/internal/base"
)

func main() {
	exe, err := os.Executable()
	if err != nil {
		log.Fatal("Не удалось определить каталог программы: ", err)
	}
	path := flag.String("db", filepath.Join(filepath.Dir(exe), "gorig-base.sqlite"), "путь к файлу базы данных")
	port := flag.String("port", "8765", "локальный порт HTTP")
	flag.Parse()
	store, err := base.Open(*path)
	if err != nil {
		log.Fatal("Не удалось открыть базу: ", err)
	}
	defer store.Close()
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", *port))
	if err != nil {
		log.Fatal("Не удалось открыть порт: ", err)
	}
	address := "http://" + listener.Addr().String() + "/"
	fmt.Printf("GoRig Base\nБаза: %s\nОткройте в браузере: %s\nДля остановки используйте меню значка в трее или Ctrl+C.\n", *path, address)
	server := &http.Server{Handler: base.Handler(store), ReadHeaderTimeout: 10 * time.Second}
	if err := runServer(server, listener, address); err != nil {
		log.Print("Ошибка работы программы: ", err)
	}
}
