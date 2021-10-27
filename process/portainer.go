package process

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"io/ioutil"
	"net/http"
)

const (
	contentType            = "application/json"
	authenticationEndpoint = "http://%s/api/auth"
	dockerEndpoint         = "tcp://%s/api/endpoints/%s/docker"
)

type portainer struct {
	docker   docker
	address  string
	username string
	password string
	header   map[string]string
}

// NewPortainer creates a new portainer process that manages a docker container
func NewPortainer(containerName, address, endpointID, username, password string) (Process, error) {
	baseURL := fmt.Sprintf(dockerEndpoint, address, endpointID)
	header := map[string]string{}
	cli, err := client.NewClientWithOpts(
		client.WithHost(baseURL),
		client.WithScheme("http"),
		client.WithAPIVersionNegotiation(),
		client.WithHTTPHeaders(header),
	)
	if err != nil {
		return nil, err
	}

	return portainer{
		docker: docker{
			client:        cli,
			containerName: "/" + containerName,
		},
		address:  address,
		username: username,
		password: password,
		header:   header,
	}, nil
}

func (portainer portainer) Start() error {
	err := portainer.docker.Start()
	if err == nil {
		return nil
	}

	if !isUnauthorized(err) {
		return err
	}

	if err := portainer.authenticate(); err != nil {
		return fmt.Errorf("could not authorize; %s", err)
	}

	return portainer.docker.Start()
}

func (portainer portainer) Stop() error {
	err := portainer.docker.Stop()
	if err == nil {
		return nil
	}

	if !isUnauthorized(err) {
		return err
	}

	if err := portainer.authenticate(); err != nil {
		return fmt.Errorf("could not authorize; %s", err)
	}

	return portainer.docker.Stop()
}

func (portainer portainer) IsRunning() (bool, error) {
	isRunning, err := portainer.docker.IsRunning()
	if err == nil {
		return isRunning, nil
	}

	if !isUnauthorized(err) {
		return false, err
	}

	if err := portainer.authenticate(); err != nil {
		return false, fmt.Errorf("could not authorize; %s", err)
	}

	return portainer.docker.IsRunning()
}

func isUnauthorized(err error) bool {
	return errdefs.GetHTTPErrorStatusCode(err) == http.StatusUnauthorized
}

func (portainer *portainer) authenticate() error {
	var credentials = struct {
		Username string `json:"Username"`
		Password string `json:"Password"`
	}{
		Username: portainer.username,
		Password: portainer.password,
	}

	bodyJSON, err := json.Marshal(credentials)
	if err != nil {
		return err
	}

	url := fmt.Sprintf(authenticationEndpoint, portainer.address)
	response, err := http.Post(url, contentType, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return errors.New(http.StatusText(response.StatusCode))
	}

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	var jwtResponse = struct {
		JWT string `json:"jwt"`
	}{}

	if err := json.Unmarshal(data, &jwtResponse); err != nil {
		return err
	}

	portainer.header["Authorization"] = fmt.Sprintf("Bearer %s", jwtResponse.JWT)
	return nil
}
