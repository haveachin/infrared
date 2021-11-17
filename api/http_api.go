package api

import (
	"fmt"
	"github.com/haveachin/infrared"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var configPath = "./configs"

// ListenAndServe StartWebserver Start Webserver if environment variable "api-enable" is set to true
func ListenAndServe(methodConfigPath string, apiBind string) {
	configPath = methodConfigPath

	fmt.Println("Starting WebAPI on " + apiBind)
	router := chi.NewRouter()
	router.Use(middleware.Logger)

	router.Post("/proxies", addProxy)
	router.Post("/proxies/{fileName}", addProxyWithName)
	router.Delete("/proxies/{file}", removeProxy)

	err := http.ListenAndServe(apiBind, router)
	if err != nil {
		log.Fatal(err)
		return
	}
}

func addProxy(w http.ResponseWriter, r *http.Request) {
	rawData, err := ioutil.ReadAll(r.Body)
	if err != nil || string(rawData) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	jsonIsValid := checkJSONAndRegister(rawData, "")
	if jsonIsValid {
		w.WriteHeader(http.StatusOK)
		return
	} else {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("{'error': 'domainName and proxyTo could not be found'}"))
		return
	}
}

func addProxyWithName(w http.ResponseWriter, r *http.Request) {
	fileName := chi.URLParam(r, "fileName")

	rawData, err := ioutil.ReadAll(r.Body)
	if err != nil || string(rawData) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	jsonIsValid := checkJSONAndRegister(rawData, fileName)
	if jsonIsValid {
		w.WriteHeader(http.StatusOK)
		return
	} else {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("{'error': 'domainName and proxyTo could not be found'}"))
		return
	}
}

func removeProxy(w http.ResponseWriter, r *http.Request) {
	file := chi.URLParam(r, "file")
	fmt.Println(file)

	err := os.Remove(configPath + "/" + file)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
}

// Helper method to check for domainName and proxyTo in a given JSON array
// If the filename is empty the domain will be used as the filename - files with the same name will be overwritten
func checkJSONAndRegister(rawData []byte, filename string) (successful bool) {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "infraredTmpConfig_")
	if err != nil {
		log.Fatal(err)
		return false
	}

	fmt.Println(tmpFile.Name())

	err = os.WriteFile(tmpFile.Name(), rawData, 0644)
	if err != nil {
		return false
	}

	var cfg infrared.ProxyConfig
	if err := cfg.LoadFromPath(tmpFile.Name()); err != nil {
		return false
	}

	path := configPath + "/" + filename
	// If fileName is empty use domainName as filename
	if filename == "" {
		path = configPath + "/" + cfg.DomainName
	}

	err = os.WriteFile(path, rawData, 0644)
	if err != nil {
		return false
	}

	return true
}
