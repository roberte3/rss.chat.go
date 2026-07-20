BINARY := rss-chat

.PHONY: build run tidy clean

build:
	go build -o $(BINARY) .

run:
	go run .

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
