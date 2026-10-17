BINARY := pdx-portrait-creator
PACKAGE := ./cmd/pdx-portrait-creator

ifeq ($(OS),Windows_NT)
	BINARY_NAME := $(BINARY).exe
	PLATFORM := windows
else
	BINARY_NAME := $(BINARY)
	PLATFORM := linux
endif

OUTPUT := bin/$(PLATFORM)/$(BINARY_NAME)

.PHONY: build release run test uitest gametest vet fmt tidy clean


## build: compile a development binary
build:
	go build -o $(OUTPUT) $(PACKAGE)

## release: compile an optimised binary without a console window on Windows
release:
ifeq ($(OS),Windows_NT)
	go build -trimpath -ldflags "-s -w -H windowsgui" -o $(OUTPUT) $(PACKAGE)
else
	go build -trimpath -ldflags "-s -w" -o $(OUTPUT) $(PACKAGE)
endif

## run: build and start the application
run: build
	$(OUTPUT)

test:
	go test ./...

## uitest: drive the real application in a hidden window; needs a display
uitest:
	go test -tags uitest -count=1 ./...

## gametest: also run the tests against a real installation, for example
##   make gametest PDX_GAME_DIR="C:/Steam/steamapps/common/Victoria 3/game"
gametest:
	PDX_GAME_DIR="$(PDX_GAME_DIR)" go test -tags uitest -count=1 -v ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy

clean:
	rm -rf bin
