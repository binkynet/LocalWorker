all:
	go build .
	go build -tags no_errgo .

test:
	go test -v ./...
	go test -v -tags no_errgo ./...
