version = $(shell tomlq -r '.tool.commitizen.version' .cz.toml)

all: build push deploy

build: tidy test
	docker build -t registry.schidlow.ski/filen-mirror:$(version) .

tidy:
	go mod tidy

test:
	go test ./... -v

bench:
	go test -bench=. -benchmem -memprofile memprofile.profile -cpuprofile profile.profile ./pkg/filedb && \
	go tool pprof -http=":8080" profile.profile

push:
	docker push registry.schidlow.ski/filen-mirror:$(version)
	
deploy:
	cat application.yaml | VERSION=$(version) envsubst | kubectl apply -f -