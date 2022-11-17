ESBUILD := $(shell go env GOPATH)/bin/esbuild

all: js/rpc.min.js

$(ESBUILD):
	go install github.com/evanw/esbuild/cmd/esbuild@latest

js/rpc.min.js: js/rpc.js $(ESBUILD)
	$(ESBUILD) js/rpc.js --target=es2020 --minify --format=esm --bundle --outfile=js/rpc.min.js

.PHONY: js/rpc.js