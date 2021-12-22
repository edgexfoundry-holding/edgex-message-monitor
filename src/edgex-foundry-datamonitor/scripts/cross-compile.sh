#!/bin/bash
go install github.com/fyne-io/fyne-cross@latest
fyne-cross linux -app-id edgex-datamonitor -arch=amd64 ./cmd/app
fyne-cross windows -app-id edgex-datamonitor -arch=amd64 ./cmd/app

