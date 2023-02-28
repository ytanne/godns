BINFILE = "godns"

build:
	@echo "Building binary"
	@go build -o $(BINFILE) main.go

clean:
	@echo "Cleaning"
	@if find . -name $(BINFILE) -print -quit | grep -q .; then rm $(BINFILE); fi
