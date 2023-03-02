BINFILE = "godns"

build:
	@echo "Building binary"
	@go build -o bin/$(BINFILE) cmd/main.go

clean:
	@echo "Cleaning"
	@rm bin/$(BINFILE)

test:
	@echo "Starting testing"
	@go test -cover -timeout 10s ./...
