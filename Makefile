
install:
	go install github.com/Soeky/pomo@latest

build:
	go build -o ~/go/bin/pomo

run:
	go run .

clean:
	rm -f pomo
