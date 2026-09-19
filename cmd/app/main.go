// MTGA Farm Bot — Go 重写版入口。
//
// 启动本地 HTTP 服务，并自动打开浏览器控制台页面。
// 窗口捕获等业务逻辑通过 /api/* 提供给前端调用。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/flourbrain/mtga-farm-bot/internal/server"
	"github.com/flourbrain/mtga-farm-bot/internal/windpi"
)

func main() {
	windpi.EnableProcessPhysical()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := server.Run(ctx, server.Options{
		Addr:        "127.0.0.1:17888",
		OpenBrowser: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "服务异常退出: %v\n", err)
		os.Exit(1)
	}
}
