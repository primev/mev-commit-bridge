.PHONY: relayer user_cli
export CGO_ENABLED=0

relayer: bin
	go build -o bin/relayer ./cmd/relayer

user-cli: bin
	go build -o bin/user_cli ./cmd/user_cli

user-emulator: bin
	go build -o bin/user_emulator ./emulator

bin:
	mkdir -p $@

up:
	docker-compose up --build -d

up-agent:
	docker-compose --profile dd_agent up --build -d

down:
	docker-compose --profile dd_agent down
