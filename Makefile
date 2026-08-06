BINARY := loan-processor
CMD    := ./cmd/server

.PHONY: build run test vet tidy clean

build:
	go build -o bin/$(BINARY) $(CMD)

run:
	go run $(CMD)

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin
