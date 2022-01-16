package main

import "ecos/gateway"

func main() {
	router := gateway.NewRouter()
	_ = router.Run(":3267")
}
