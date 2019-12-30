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
)

type portainer struct {
	client      http.Client
	token       string
	address     string
	endpointID  string
	containerID string
	isRunning   bool
}

// NewPortainer creates a new portainer process that manages a docker container
func NewPortainer(address, endpointID, containerID, username, password string) (Process, error) {
	proc := portainer{
		client: http.Client{
			Timeout: defaultTimeout,
		},
		address:     address,
		endpointID:  endpointID,
		containerID: containerID,
	}

	if err := proc.authenticate(username, password); err != nil {
		return nil, err
	}

	return proc, nil
}

func (proc portainer) Start() error {
	return proc.do(startContainerEndpoint)
}

func (proc portainer) Stop() error {
	return proc.do(stopContainerEndpoint)
}

func (proc portainer) do(url string) error {
	url = fmt.Sprintf(url, proc.address, proc.endpointID, proc.containerID)

	request, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}

	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", proc.token))

	response, err := proc.client.Do(request)
	if err != nil {
		return err
	}

	switch response.StatusCode {
	case http.StatusNotFound:
		return errors.New("no such container")
	case http.StatusInternalServerError:
		return errors.New("server error: " + url)
	}

	return nil
}

func (proc *portainer) authenticate(username, password string) error {
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
