package main

import (
	"log"
	"os"

	"github.com/zhengtianbao/promproxy/config"
	"github.com/zhengtianbao/promproxy/middleware"
	"github.com/zhengtianbao/promproxy/server"
	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	os.Exit(run())
}

func run() int {
	configFile := "config.yaml"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	log.SetOutput(&lumberjack.Logger{
    Filename:   "promproxy.log",
    MaxSize:    100,
    MaxBackups: 3,
    MaxAge:     28,
    Compress:   true,
})

	log.Printf("config: %s", configFile)
	config, err := config.LoadFile(configFile)
	if err != nil {
		log.Printf("err: %s", err)
		os.Exit(1)
	}

	server := server.NewProxyServer(config)
	middlewares := []middleware.Middleware{
		middleware.NewLabelValidateMiddleware(config.Rules.AllowedSpaces),
		middleware.NewTimeValidateMiddleware(),
		middleware.NewFunctionValidateMiddleware(),
		middleware.NewQueryRangeMiddleware(),
	}
	server.RegisterMiddlewares(middlewares...)
	if err := server.Start(); err != nil {
		log.Printf("err: %s", err)
	}
	return 0
}
