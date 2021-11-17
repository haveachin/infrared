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
	router.Post("/proxies/{fileName}", addProxyWithName)
	router.Delete("/proxies/{file}", removeProxy)

	err := http.ListenAndServe(apiBind, router)
	if err != nil {
		log.Fatal(err)
	}
}

func addProxy(w http.ResponseWriter, r *http.Request) {
	rawData, err := ioutil.ReadAll(r.Body)
	if err != nil || string(rawData) == "" {
		w.WriteHeader(400)
	}

	jsonIsValid, jsonData := checkForRequiredObjects(rawData)
	if jsonIsValid {
		proxyName := jsonData["domainName"]
		filePath := ConfigPath + "/" + fmt.Sprint(proxyName)
		createProxyFile(filePath, rawData)
	} else {
		w.WriteHeader(400)
		w.Write([]byte("{'error': 'domainName and proxyTo were not found'}"))
	}
}

func addProxyWithName(w http.ResponseWriter, r *http.Request) {
	fileName := strings.TrimPrefix(r.URL.Path, "/proxies/")

	rawData, err := ioutil.ReadAll(r.Body)
	if err != nil || string(rawData) == "" {
		w.WriteHeader(400)
	}

	jsonIsValid, _ := checkForRequiredObjects(rawData)
	if jsonIsValid {
		filePath := ConfigPath + "/" + fileName
		createProxyFile(filePath, rawData)
	} else {
		w.WriteHeader(400)
		w.Write([]byte("{'error': 'domainName and proxyTo could not be found'}"))
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

// Helper method to check for domainName and proxyTo in a given JSON array
func checkForRequiredObjects(rawData []byte) (successful bool, jsonData map[string]interface{}) {
	var result map[string]interface{}
	err := json.Unmarshal(rawData, &result)
	if err != nil {
		return false, nil
	}

	return result["domainName"] != nil && result["proxyTo"] != nil, result
}

// Method to create proxy file in config directory
func createProxyFile(path string, json []byte) {
	err := os.WriteFile(path, json, 0644)
	if err != nil {
		log.Fatal(err)
	}
}
