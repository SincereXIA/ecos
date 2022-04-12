package main

import "ecos/gateway/router"

func main() {
	r := router.NewRouter(router.DefaultConfig)
	_ = r.Run(":3267")
}
