dev:
	go run ./cmd/infrared -config-path=./config/config.dev.yml

docker-dev:
	docker-compose -p infrared -f deployments/docker-compose.dev.yml build --no-cache --force-rm
	docker-compose -p infrared -f deployments/docker-compose.dev.yml up --force-recreate --remove-orphans