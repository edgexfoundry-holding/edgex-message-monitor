#!/bin/bash

go mod tidy
go build -o ./bin/edgex-foundry-datamonitor ./cmd/app/main.go
