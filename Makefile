dev-run:
	go run ./cmd/infrared -config-path=./config/config.dev.yml

docker-dev-build:
	docker-compose -p infrared -f deployments/docker-compose.dev.yml build --no-cache --force-rm

docker-dev-run:
	docker-compose -p infrared -f deployments/docker-compose.dev.yml up --force-recreate --remove-orphans

docker-dev:
	make docker-dev-build
	make docker-dev-run