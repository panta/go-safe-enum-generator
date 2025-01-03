BINARY_NAME=go-safe-enum-generator

.PHONY: all
all: build

# Build command
.PHONY: build
build $(BINARY_NAME):
	go build -o $(BINARY_NAME) -ldflags="-w -s" main.go

# Clean command (optional)
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)

install: $(BINARY_NAME)
	cp -rp $(BINARY_NAME) ${GOPATH}/bin/

.PHONY: build clean install
