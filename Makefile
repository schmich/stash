ifneq ($(PLATFORM),)
	platform=$(PLATFORM)
else
	ifeq ($(OS),Windows_NT)
		platform=windows
	else
		platform=$(shell uname -s | tr 'A-Z' 'a-z')
	endif
endif

ifeq ($(platform),windows)
	ext=.exe
else
	ext=
endif

build-image=stash/build
ensure-image=docker image inspect $(build-image) &>/dev/null || make image
docker=docker run --rm -v `pwd`:/src -w /src -e GOCACHE=/src/.cache
source=cmd/*.go crypt/*.go identifier/*.go storage/*.go vendor

stash$(ext): $(source)
	@$(ensure-image)
	$(docker) -e GOOS=$(platform) $(build-image) go build -o $@ -mod vendor -ldflags="-s -w" github.com/schmich/stash/cmd

build/debug/darwin/stash: $(source)
	@$(ensure-image)
	$(docker) -e GOOS=darwin $(build-image) go build -o $@ -mod vendor -ldflags="-s -w" github.com/schmich/stash/cmd

build/debug/linux/stash: $(source)
	@$(ensure-image)
	$(docker) -e GOOS=linux $(build-image) go build -o $@ -mod vendor -ldflags="-s -w" github.com/schmich/stash/cmd

build/debug/windows/stash.exe: $(source)
	@$(ensure-image)
	$(docker) -e GOOS=windows $(build-image) go build -o $@ -mod vendor -ldflags="-s -w" github.com/schmich/stash/cmd

build/release/darwin/stash: build/debug/darwin/stash
	@$(ensure-image)
	mkdir -p build/release/darwin
	$(docker) $(build-image) upx --best --ultra-brute -o$@ $<

build/release/linux/stash: build/debug/linux/stash
	@$(ensure-image)
	mkdir -p build/release/linux
	$(docker) $(build-image) upx --best --ultra-brute -o$@ $<

build/release/windows/stash.exe: build/debug/windows/stash.exe
	@$(ensure-image)
	mkdir -p build/release/windows
	$(docker) $(build-image) upx --best --ultra-brute -o$@ $<

vendor: go.mod go.sum
	@$(ensure-image)
	$(docker) $(build-image) go mod vendor

.PHONY: image
image:
	docker build -t $(build-image) .

darwin-debug: build/debug/darwin/stash
linux-debug: build/debug/linux/stash
windows-debug: build/debug/windows/stash.exe
debug: darwin-debug linux-debug windows-debug

darwin-release: build/release/darwin/stash
linux-release: build/release/linux/stash
windows-release: build/release/windows/stash.exe
release: darwin-release linux-release windows-release