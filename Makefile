proto-v1:
	protoc --go_out=. --go-grpc_out=. api/proto/v1/*.proto

keygen:
	go run cmd/keygen/main.go

migrate:
	go run cmd/migrate/main.go

build-ledger:
	go build -o bin/ledger cmd/ledger/main.go

build-migrate:
	go build -o bin/migrate cmd/migrate/main.go

