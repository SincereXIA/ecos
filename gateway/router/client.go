package router

import (
	"ecos/client"
	"ecos/client/config"
	"ecos/utils/logger"
)

var Client *client.Client

func InitClient(config config.ClientConfig) {
	c, err := client.New(&config)
	if err != nil {
		logger.Fatalf("Init client for gateway failed, err: %v", err)
	}
	logger.Warningf("Init client for gateway success")
	Client = c
}
