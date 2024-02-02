.PHONY: docs

test:
	go test -race -timeout 10s ./...

build:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o ./out/infrared ./cmd/infrared

all: test build

run: build
	./out/infrared -w .dev/infrared

bench:
	go test -bench=. -run=x -benchmem -memprofile mem.prof -cpuprofile cpu.prof -benchtime=10s > 0.bench
	go tool pprof cpu.prof

dev:
	docker compose -f deployments/docker-compose.dev.yml -p infrared up --force-recreate --remove-orphans

dos:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o ./out/dos ./tools/dos
	./out/dos

malpk:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o ./out/malpk ./tools/malpk
	./out/malpk

docs:
	cd ./docs && npm i && npm run docs:dev

lint:
	golangci-lint run
