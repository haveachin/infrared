tests:
	go install github.com/golang/mock/mockgen@v1.6.0
	go generate ./...
	go test -race -timeout 10s ./...

envdev:
	docker-compose -p infrared -f deployments/docker-compose.dev.yml up --force-recreate --remove-orphans

envtest:
	docker-compose -p infrared -f deployments/docker-compose.test.yml build --no-cache --force-rm
	docker-compose -p infrared -f deployments/docker-compose.test.yml up --force-recreate --remove-orphans

run:
	go run -race . -c config.yml -w dev/ -e dev

plantuml:
	plantuml -tsvg *.md docs/*.md docs/plugins/*.md docs/*.plantuml docs/plugins/*.plantuml
