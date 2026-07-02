#!/usr/bin/make -f

.PHONY: delete-images

SHELL = /bin/bash

PROJECT ?= finance-app-dev-491520
data ?=

api:
	GOWORK=off GOOS=darwin GOARCH=arm64 go build -o ../../../../bin/financial-service ./cmd/api

worker:
	GOWORK=off GOOS=darwin GOARCH=arm64 go build -o ../../../../bin/financial-worker ./cmd/worker

sync-data:
	@if [ -z "$(data)" ]; then echo "data is required. Usage: make syncdata data=<fixture-name>"; exit 1; fi
	GOWORK=off go run ./cmd/tools/seed-sync-data --project="$(PROJECT)" --file="testdata/sync-data/$(data).yaml"

run-alert-evaluator:
	gcloud run jobs execute alert-evaluator --region=us-central1 --project="$(PROJECT)"

seed-alert-event:
	GOWORK=off GOOS=darwin GOARCH=arm64 go build -o ../../../../bin/seed-alert-event ./cmd/tools/seed-alert-event

gcpbootstrap:
	@if [ -z "$(PROJECT)" ]; then echo "PROJECT is required. Usage: make gcpbootstrap PROJECT=<gcp-project-id>"; exit 1; fi
	gcloud services enable cloudresourcemanager.googleapis.com --project $(PROJECT)
	gcloud services enable serviceusage.googleapis.com --project $(PROJECT)
	gcloud services enable compute.googleapis.com --project $(PROJECT)

delete-images:
	@if [ -z "$(name)" ]; then \
		echo "Usage: make delete-image name=<image-name>"; \
		exit 1; \
	fi
	@docker images --format "{{.Repository}}:{{.Tag}} {{.ID}}" | \
		grep "$(name)" | \
		awk '{print $$2}' | \
		xargs docker rmi

taxonomy:
	go generate ./internal/taxonomy
