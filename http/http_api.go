package http

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"os"
	"strings"

	"github.com/haveachin/infrared"
)

var ConfigPath = "./configs"
var ProxyGateway = infrared.Gateway{}

// StartWebserver Start Webserver if environment variable "api-enable" is set to true
func StartWebserver(configPath string, gateway infrared.Gateway) {
	ProxyGateway = gateway
	ConfigPath = configPath
	if getEnv("API-ENABLED", "false") == "true" {
		apiBind := getEnv("API-BIND", "127.0.0.1:8080")

		fmt.Println("Starting WebAPI on " + apiBind)
		server := gin.Default()
		gin.SetMode(gin.ReleaseMode)

		server.POST("/proxies/", addProxy)
		server.DELETE("/proxies/:file/", removeProxy)

		err := server.Run(apiBind)
		if err != nil {
			panic(err)
		}
	}
}

func addProxy(c *gin.Context) {
	jsonData, err := ioutil.ReadAll(c.Request.Body)
	if err != nil || string(jsonData) == "" {
		c.AbortWithStatus(400)
	}

	var result map[string]interface{}
	err = json.Unmarshal([]byte(jsonData), &result)
	if err != nil {
		c.AbortWithError(400, err)
	}

	if result["domainName"] != nil && result["proxyTo"] != nil {
		proxyName := result["domainName"]
		filePath := ConfigPath + "/" + fmt.Sprint(proxyName)

		err := os.WriteFile(filePath, jsonData, 0644)
		if err != nil {
			c.AbortWithError(500, err)
		}

		conf, err := infrared.NewProxyConfigFromPath(filePath)
		if err != nil {
			c.AbortWithError(500, err)
		}

		ProxyGateway.RegisterProxy(&infrared.Proxy{
			Config: conf,
		})

	} else {
		c.AbortWithStatusJSON(400, "{'error': 'domainName and proxyTo were not found'}")
	}

}

func removeProxy(c *gin.Context) {
	file := c.Param("file")
	successful := false

	ProxyGateway.Proxies.Range(func(k, v interface{}) bool {
		otherProxy := v.(*infrared.Proxy)
		if strings.ToLower(otherProxy.Config.DomainName) == file {
			ProxyGateway.CloseProxy(otherProxy.UID())
			err := os.Remove(ConfigPath + "/" + file)
			if err != nil {
				c.AbortWithError(400, err)
			}
			c.Status(200)
			successful = true
		}
		return true
	})

	if successful == false {
		c.AbortWithStatusJSON(400, "{'error': 'file not found'}")
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
