package config

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Config 服务配置
type Config struct {
	Port            string        // 监听端口
	DataDir         string        // 数据目录（SQLite 数据库与上传文件）
	JWTSecret       []byte        // 登录凭证签名密钥（首次启动生成并持久化）
	CleanupInterval time.Duration // 数据清理任务周期
}

// Load 读取配置：命令行参数优先，其次环境变量，最后默认值
func Load() Config {
	port := flag.String("port", env("PORT", "8080"), "监听端口")
	dataDir := flag.String("data", env("DATA_DIR", "./data"), "数据目录")
	cleanupInterval := flag.Duration("cleanup-interval", envDuration("CLEANUP_INTERVAL", 10*time.Minute), "数据清理任务周期")
	flag.Parse()

	abs, err := filepath.Abs(*dataDir)
	if err != nil {
		abs = *dataDir
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	return Config{
		Port:            *port,
		DataDir:         abs,
		JWTSecret:       loadOrCreateSecret(filepath.Join(abs, "jwt.key")),
		CleanupInterval: *cleanupInterval,
	}
}

// loadOrCreateSecret 读取持久化的 JWT 密钥；不存在时生成并写入
func loadOrCreateSecret(path string) []byte {
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		return b
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		log.Fatalf("生成 JWT 密钥失败: %v", err)
	}
	secret := []byte(hex.EncodeToString(buf))
	if err := os.WriteFile(path, secret, 0o600); err != nil {
		log.Fatalf("持久化 JWT 密钥失败: %v", err)
	}
	return secret
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
