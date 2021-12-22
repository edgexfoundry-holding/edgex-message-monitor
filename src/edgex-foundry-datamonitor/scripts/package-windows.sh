#!/bin/bash
go install fyne.io/fyne/v2/cmd/fyne@latest
fyne package -os windows --src ./cmd/app
