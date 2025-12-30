


build: tidy test
	docker build -t filen-mirror .

tidy:
	go mod tidy

test:
	go test ./... -v

bench:
	go test -bench=. -benchmem -memprofile memprofile.profile -cpuprofile profile.profile ./pkg/filedb && \
	go tool pprof -http=":8080" profile.profile