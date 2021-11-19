package api

import (
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/haveachin/infrared"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

// ListenAndServe StartWebserver Start Webserver if environment variable "api-enable" is set to true
func ListenAndServe(configPath string, apiBind string) {
	fmt.Println("Starting WebAPI on " + apiBind)
	router := chi.NewRouter()
	router.Use(middleware.Logger)

	router.Post("/proxies", addProxy(configPath))
	router.Post("/proxies/{fileName}", addProxyWithName(configPath))
	router.Delete("/proxies/{fileName}", removeProxy(configPath))

	err := http.ListenAndServe(apiBind, router)
	if err != nil {
		log.Fatal(err)
		return
	}
}

func addProxy(configPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawData, err := ioutil.ReadAll(r.Body)
		if err != nil || string(rawData) == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		jsonIsValid := checkJSONAndRegister(rawData, "", configPath)
		if jsonIsValid {
			w.WriteHeader(http.StatusOK)
			return
		} else {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("{'error': 'domainName and proxyTo could not be found'}"))
			return
		}
	}
}

func addProxyWithName(configPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fileName := chi.URLParam(r, "fileName")

		rawData, err := ioutil.ReadAll(r.Body)
		if err != nil || string(rawData) == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		jsonIsValid := checkJSONAndRegister(rawData, fileName, configPath)
		if jsonIsValid {
			w.WriteHeader(http.StatusOK)
			return
		} else {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("{'error': 'domainName and proxyTo could not be found'}"))
			return
		}
	}
}

func removeProxy(configPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file := chi.URLParam(r, "fileName")
		fmt.Println(file)

		err := os.Remove(configPath + "/" + file)
		if err != nil {
			w.WriteHeader(http.StatusNoContent)
			w.Write([]byte(err.Error()))
			return
		}
	}
}

// Helper method to check for domainName and proxyTo in a given JSON array
// If the filename is empty the domain will be used as the filename - files with the same name will be overwritten
func checkJSONAndRegister(rawData []byte, filename string, configPath string) (successful bool) {
	var cfg infrared.ProxyConfig
	err := json.Unmarshal(rawData, &cfg)
	if err != nil {
		fmt.Println(err)
		return false
	}

	if cfg.DomainName == "" || cfg.ProxyTo == "" {
		return false
	}

	path := configPath + "/" + filename
	// If fileName is empty use domainName as filename
	if filename == "" {
		path = configPath + "/" + cfg.DomainName
	}

	err = os.WriteFile(path, rawData, 0644)
	if err != nil {
		fmt.Println(err)
		return false
	}

	return true
}
