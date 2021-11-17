package http

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var ConfigPath = "./configs"

// StartWebserver Start Webserver if environment variable "api-enable" is set to true
func StartWebserver(configPath string, apiBind string) {
	ConfigPath = configPath

	fmt.Println("Starting WebAPI on " + apiBind)
	router := chi.NewRouter()
	router.Use(middleware.Logger)

	router.Post("/proxies", addProxy)
	router.Delete("/proxies/{file}", removeProxy)

	err := http.ListenAndServe(apiBind, router)
	if err != nil {
		log.Fatal(err)
	}
}

func addProxy(w http.ResponseWriter, r *http.Request) {
	jsonData, err := ioutil.ReadAll(r.Body)
	if err != nil || string(jsonData) == "" {
		w.WriteHeader(400)
	}

	var result map[string]interface{}
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		w.WriteHeader(400)
	}

	if result["domainName"] != nil && result["proxyTo"] != nil {
		proxyName := result["domainName"]
		filePath := ConfigPath + "/" + fmt.Sprint(proxyName)

		err := os.WriteFile(filePath, jsonData, 0644)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
		}

	} else {
		w.WriteHeader(400)
		w.Write([]byte("{'error': 'domainName and proxyTo were not found'}"))
	}

}

func removeProxy(w http.ResponseWriter, r *http.Request) {
	file := strings.TrimPrefix(r.URL.Path, "/proxies/")
	fmt.Println(file)

	err := os.Remove(ConfigPath + "/" + file)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}
}
