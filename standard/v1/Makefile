.PHONY: relayer user_cli
export CGO_ENABLED=0

all: relayer user_cli

relayer: bin
	go build -o bin/relayer ./cmd/relayer

user_cli: bin
	go build -o bin/user_cli ./cmd/user_cli

bin:
	mkdir -p $@

up:
	docker-compose up --build -d

down:
	docker-compose down
