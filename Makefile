.PHONY: assets fmt clean client-assets server-assets

assets: client-assets server-assets

fmt:
	go fmt ngrok/...

client-assets:
	go get github.com/inconshreveable/go-bindata
	GOOS="" GOARCH="" go install github.com/inconshreveable/go-bindata
	bin/go-bindata -o src/ngrok/client/assets assets/client

server-assets:
	go get github.com/inconshreveable/go-bindata
	GOOS="" GOARCH="" go install github.com/inconshreveable/go-bindata
	bin/go-bindata -o src/ngrok/server/assets assets/server

clean:
	go clean -i -r ngrok/...
