
install:
	go install github.com/Soeky/pomo@latest

build:
	go build -o ~/go/bin/pomo

run:
	go run .

test:
	go test ./...

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

test-race:
	go test -race ./...

vet:
	go vet ./...

coverage-gate:
	./scripts/coverage_gate.sh 85 ./internal/db ./internal/parse ./internal/session ./internal/stats ./internal/status

clean:
	rm -f pomo
