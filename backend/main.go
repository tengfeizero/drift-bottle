package main

import (
	"log"

	"driftbottle/internal/config"
	"driftbottle/internal/server"
)

func main() {
	cfg := config.Load()

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("服务初始化失败: %v", err)
	}
	if err := srv.Start(); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
