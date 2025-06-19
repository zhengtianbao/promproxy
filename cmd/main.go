package main

import (
	"log"
	"os"

	"github.com/zhengtianbao/promproxy/config"
	"github.com/zhengtianbao/promproxy/middleware"
	"github.com/zhengtianbao/promproxy/server"
)

func main() {
	os.Exit(run())
}

func run() int {
	configFile := "config.yaml"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

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
	}
	server.RegisterMiddlewares(middlewares...)
	if err := server.Start(); err != nil {
		log.Printf("err: %s", err)
	}
	return 0
}
