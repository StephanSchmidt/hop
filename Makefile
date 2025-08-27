hop: go-imports
	go build -o hop cmd/hop/main.go
	chmod 755 hop

go-imports:
	go run golang.org/x/tools/cmd/goimports -w .

clean:
	go clean -cache -i
	
lint:
	go run honnef.co/go/tools/cmd/staticcheck ./...
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run ./...

upgrade-deps:
	go get -u ./...
	go mod tidy
	go run gotest.tools/gotestsum ./...