package process

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

const (
	contentType            = "application/json"
	authenticationEndpoint = "http://%s/api/auth"
	startContainerEndpoint = "http://%s/api/endpoints/%s/docker/containers/%s/start"
	stopContainerEndpoint  = "http://%s/api/endpoints/%s/docker/containers/%s/stop"
	jsonContainerEndpoint  = "http://%s/api/endpoints/%s/docker/containers/%s/json"
)

type portainerProcess struct {
	client      http.Client
	token       string
	address     string
	endpointID  string
	containerID string
	isRunning   bool
}

// NewPortainer creates a new portainer process that manages a docker container
func NewPortainer(address, endpointID, containerID, username, password string) (Process, error) {
	proc := portainerProcess{
		client: http.Client{
			Timeout: contextTimeout,
		},
		address:     address,
		endpointID:  endpointID,
		containerID: containerID,
		isRunning:   false,
	}

	if err := proc.authenticate(username, password); err != nil {
		return nil, err
	}

	return proc, nil
}

func (proc portainerProcess) Start() error {
	if _, err := proc.do(http.MethodPost, startContainerEndpoint); err != nil {
		return err
	}

	return nil
}

func (proc portainerProcess) Stop() error {
	if _, err := proc.do(http.MethodPost, stopContainerEndpoint); err != nil {
		return err
	}

	return nil
}

func (proc portainerProcess) IsRunning() bool {
	response, err := proc.do(http.MethodGet, jsonContainerEndpoint)
	if err != nil {
		return false
	}

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return false
	}

	state := struct {
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
	}{
		State: struct {
			Running bool `json:"Running"`
		}{
			Running: false,
		},
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return false
	}

	return state.State.Running
}

func (proc portainerProcess) do(method, url string) (*http.Response, error) {
	url = fmt.Sprintf(url, proc.address, proc.endpointID, proc.containerID)

	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", proc.token))

	response, err := proc.client.Do(request)
	if err != nil {
		return nil, err
	}

	switch response.StatusCode {
	case http.StatusNotFound:
		return nil, errors.New("no such container")
	case http.StatusInternalServerError:
		return nil, errors.New("server error: " + url)
	}

	return response, nil
}

func (proc *portainerProcess) authenticate(username, password string) error {
	credentials := struct {
		Username string `json:"Username"`
		Password string `json:"Password"`
	}{
		Username: username,
		Password: password,
	}

	bodyJSON, err := json.Marshal(credentials)
	if err != nil {
		return err
	}

	url := fmt.Sprintf(authenticationEndpoint, proc.address)
	response, err := proc.client.Post(url, contentType, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return errors.New("")
	}

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	jwtResponse := struct {
		JWT string `json:"jwt"`
	}{
		JWT: "",
	}

	if err := json.Unmarshal(data, &jwtResponse); err != nil {
		return err
	}

	proc.token = jwtResponse.JWT

	return nil
}
