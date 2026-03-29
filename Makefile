#!/usr/bin/make -f

SHELL = /bin/bash

PROJECT ?=

service:
	GOOS=darwin GOARCH=arm64 go build -o ../../../../bin/financial-service cmd/api/*.go

gcpbootstrap:
	@if [ -z "$(PROJECT)" ]; then echo "PROJECT is required. Usage: make gcpbootstrap PROJECT=<gcp-project-id>"; exit 1; fi
	gcloud services enable cloudresourcemanager.googleapis.com --project $(PROJECT)
	gcloud services enable serviceusage.googleapis.com --project $(PROJECT)
	gcloud services enable compute.googleapis.com --project $(PROJECT)

taxonomy:
	go generate ./internal/taxonomy
