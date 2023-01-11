build:
	rm -rf apicompare
	go build -o apicompare ./main.go

lint:
	golangci-lint run
