package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	if cfg.selfCheck {
		if err := runSelfCheck(context.Background(), cfg); err != nil {
			return fmt.Errorf("自检失败: %w", err)
		}
		fmt.Println("自检通过：批量抽样与圈定预检、方案审批、中止处置与恢复、复验、独立放行、封存清单、幂等与完整性校验均成功")
		return nil
	}
	rt, err := buildRuntime(cfg)
	if err != nil {
		return err
	}
	serveErrors := make(chan error, 1)
	go rt.serve(serveErrors)
	fmt.Printf("磁带保存事件服务监听 %s\n", cfg.addr)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case err := <-serveErrors:
		return err
	case <-signals:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := rt.shutdown(ctx); err != nil {
			return fmt.Errorf("优雅关闭: %w", err)
		}
		return <-serveErrors
	}
}
