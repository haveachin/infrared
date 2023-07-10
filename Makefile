test:
	go test -race -timeout 10s ./...

all: test
	CGO_ENABLED=0 go build -ldflags "-s -w" -o ./out/infrared ./cmd/infrared

run: all
	./out/infrared

bench:
	go test -bench=. -run=x -benchmem -memprofile mem.prof -cpuprofile cpu.prof -benchtime=10s > 0.bench
	go tool pprof cpu.prof

dev:
	docker compose -f deployments/docker-compose.dev.yml -p infrared up --force-recreate --remove-orphans

dos:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o ./out/dos ./tools/dos
	./out/dos