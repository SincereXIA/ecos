package router

import (
	"ecos/client"
	"ecos/client/config"
	"ecos/utils/logger"
	"time"
)

var Client *client.Client

func InitClient(config config.ClientConfig) {
	var c *client.Client
	var err error
	timer := time.NewTimer(time.Minute * 5)
	for {
		select {
		case <-timer.C:
			return
		default:
		}
		c, err = client.New(&config)
		if err != nil {
			logger.Warningf("Init client for gateway failed, err: %v", err)
			time.Sleep(time.Second * 3)
			logger.Infof("Retry init client for gateway")
			continue
		}
		logger.Infof("Init client for gateway success")
		break
	}
	Client = c
}
