accural:
	@./cmd/accrual/accrual_linux_amd64 -a=:8081 -d=postgresql://gophermart:gophermart@192.168.0.27:5412/gophermart

gophermart:
	@./cmd/gophermart/gophermart

# For develop
build:
	@go build -o cmd/gophermart/gophermart cmd/gophermart/main.go

run: build
	@./cmd/gophermart/gophermart

migration_down:
	@goose -dir internal/storage/migrations postgres "host=192.168.0.27 port=5412 user=gophermart password=gophermart dbname=gophermart sslmode=disable" down