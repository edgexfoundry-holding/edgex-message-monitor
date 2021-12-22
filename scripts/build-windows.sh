cd src/edgex-foundry-datamonitor
go build -o ../../build/edgex-datamonitor cmd/app/main.go
cd ../../
mv build/edgex-datamonitor build/edgex-datamonitor.exe