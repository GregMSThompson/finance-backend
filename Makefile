#!/usr/bin/make -f

SHELL = /bin/bash

service:
	GOOS=darwin GOARCH=arm64 go build -o ../../../../bin/financial-service cmd/service/*.go
