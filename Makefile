BIN?=bin

all: build

build:
	mkdir -p $(BIN)
	GO111MODULE=on go build -o $(BIN)/controller ./cmd/controller
	GO111MODULE=on go build -o $(BIN)/agent ./cmd/agent
	GO111MODULE=on go build -o $(BIN)/relay ./cmd/relay

clean:
	rm -rf $(BIN)
