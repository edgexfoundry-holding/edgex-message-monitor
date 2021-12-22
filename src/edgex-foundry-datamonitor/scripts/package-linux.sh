#!/bin/bash
go install fyne.io/fyne/v2/cmd/fyne@latest
fyne package -os linux --src ./cmd/app
