.PHONY: build run clean deps

# Build with memory limits (gotd/td/tg is a huge auto-generated package)
build:
	GOGC=20 GOMEMLIMIT=2GiB go build -p=1 -o vimgram ./cmd/vimgram

run: build
	./vimgram

clean:
	rm -f vimgram session.json

# Fetch dependencies (also with memory limits, just in case)
deps:
	GOGC=20 GOMEMLIMIT=2GiB go mod tidy
