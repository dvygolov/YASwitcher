# YASwitcher build. Cross compiles from Linux/WSL. No CGO, no external tools
# except go-winres (only needed to embed the icon + manifest).

APP      := yas-switcher
VERSION  ?= 0.1.0
OUT      := build/$(APP).exe
LDFLAGS  := -s -w -H windowsgui -X main.version=$(VERSION)

.PHONY: all icon resources build run clean test

all: build

icon:
	go run ./tools/genicon -out build/app.ico -preview build/icon-256.png

resources: icon
	GOARCH=amd64 go-winres simply --arch amd64 --manifest gui \
	  --icon build/app.ico --product-name YASwitcher \
	  --file-description "YASwitcher - yet another window switcher" \
	  --copyright "MIT License" \
	  --product-version $(VERSION) --file-version $(VERSION)

build: resources
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUT) .
	@ls -la $(OUT)

test:
	GOOS=windows GOARCH=amd64 go vet -unsafeptr=false -composites=false ./...
	GOOS=windows GOARCH=amd64 go build ./...
	gofmt -l .

clean:
	rm -rf build rsrc_windows_*.syso
