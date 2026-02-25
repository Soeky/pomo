
install:
	go install github.com/Soeky/pomo@latest

build:
	go build -o ~/go/bin/pomo

run:
	go run .

test:
	go test ./...

test-cover:
	go test ./internal/... -coverprofile=coverage.out
	go tool cover -func=coverage.out
	./scripts/coverage_total.sh 70 ./internal/...

test-race:
	go test -race ./...

vet:
	go vet ./...

coverage-gate:
	./scripts/coverage_gate.sh 70 ./internal/db ./internal/parse ./internal/session ./internal/stats ./internal/status

coverage-total:
	./scripts/coverage_total.sh 70 ./internal/...

clean:
	rm -f pomo
