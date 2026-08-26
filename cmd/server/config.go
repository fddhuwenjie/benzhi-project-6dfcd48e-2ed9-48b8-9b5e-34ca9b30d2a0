package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	addr             string
	dataDir          string
	selfCheck        bool
	selfCheckTimeout time.Duration
}

func parseConfig(args []string) (config, error) {
	defaults := "127.0.0.1:19091"
	if portText := os.Getenv("PORT"); portText != "" {
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1024 || port > 65535 {
			return config{}, fmt.Errorf("PORT 必须是 1024 到 65535 之间的端口号")
		}
		defaults = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}
	set := flag.NewFlagSet("server", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	var cfg config
	set.StringVar(&cfg.addr, "addr", defaults, "HTTP 监听地址")
	set.StringVar(&cfg.dataDir, "data-dir", "./data", "本地数据目录")
	set.BoolVar(&cfg.selfCheck, "self-check", false, "运行真实 HTTP 全流程自检并退出")
	set.DurationVar(&cfg.selfCheckTimeout, "self-check-timeout", 20*time.Second, "自检超时时间")
	if err := set.Parse(args); err != nil {
		return config{}, err
	}
	if set.NArg() != 0 {
		return config{}, fmt.Errorf("存在无法识别的位置参数")
	}
	if err := validateAddress(cfg.addr); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(cfg.dataDir) == "" {
		return config{}, fmt.Errorf("data-dir 不能为空")
	}
	if cfg.selfCheckTimeout < time.Second || cfg.selfCheckTimeout > 2*time.Minute {
		return config{}, fmt.Errorf("self-check-timeout 必须在 1 秒到 2 分钟之间")
	}
	return cfg, nil
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("addr 必须为 host:port 格式: %w", err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return fmt.Errorf("addr 不得使用通配监听地址")
	}
	if ip := net.ParseIP(host); ip == nil && host != "localhost" {
		return fmt.Errorf("addr 主机必须是明确 IP 或 localhost")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1024 || port > 65535 {
		return fmt.Errorf("addr 端口必须在 1024 到 65535 之间")
	}
	return nil
}
