dev-run:
	go run ./cmd/infrared -config-path=./configs/config.dev.yml

dev-build-docker:
	docker-compose -p infrared -f deployments/docker-compose.dev.yml build --no-cache --force-rm

dev-run-docker:
	docker-compose -p infrared -f deployments/docker-compose.dev.yml up --force-recreate --remove-orphans

dev-docker:
	make dev-build-docker
	make dev-run-docker