proto-v1:
	protoc --go_out=. --go-grpc_out=. api/proto/v1/*.proto

migrate:
	go run cmd/migrate/main.go

build-ledger:
	go build -o bin/ledger cmd/ledger/main.go

build-migrate:
	go build -o bin/migrate cmd/migrate/main.go

