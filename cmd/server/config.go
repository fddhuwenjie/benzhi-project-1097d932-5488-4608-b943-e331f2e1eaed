package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultAddress = "127.0.0.1:19137"

type config struct {
	Address   string
	Database  string
	SelfCheck bool
}

func parseConfig() (config, error) {
	var cfg config
	flag.StringVar(&cfg.Address, "addr", "", "监听地址，例如 127.0.0.1:19137")
	flag.StringVar(&cfg.Database, "db", "accessibility_release.db", "SQLite 数据库路径")
	flag.BoolVar(&cfg.SelfCheck, "self-check", false, "运行真实 HTTP 全流程自检后退出")
	flag.Parse()
	if cfg.Address == "" {
		if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
			cfg.Address = "127.0.0.1:" + port
		} else {
			cfg.Address = defaultAddress
		}
	}
	if err := validateAddress(cfg.Address, cfg.SelfCheck); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func validateAddress(address string, selfCheck bool) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("无效 -addr: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1024 || port > 65535 {
		return fmt.Errorf("端口必须在 1024 到 65535 之间")
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("监听主机必须是 IP 地址")
	}
	if ip.IsUnspecified() {
		return fmt.Errorf("禁止绑定未指定地址 %s", host)
	}
	if selfCheck && !ip.IsLoopback() {
		return fmt.Errorf("自检地址必须是回环地址")
	}
	return nil
}
