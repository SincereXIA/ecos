package gateway

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net"
	"net/http"
)

func NewRouter() *gin.Engine {
	router := gin.Default()
	router.GET("/", hello)
	return router
}

func hello(c *gin.Context) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println(err)
		return
	}
	selfIp := ""
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				selfIp = ipnet.IP.String()
			}
		}
	}
	c.String(http.StatusOK, "ECOS EdgeNode: %s", selfIp)
}
