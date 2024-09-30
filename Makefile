export GO_EXECUTABLE_PATH := $(shell which go)

build:
	@mkdir -p ./bin && go build -o ./bin/ -v ./...

test:
	@$$GO_EXECUTABLE_PATH test -v -race ./...

coverage:
	@$$GO_EXECUTABLE_PATH test -v -race --cover --coverprofile=cover.profile ./...

coverage-report: coverage
	@$$GO_EXECUTABLE_PATH tool cover -html=cover.profile
