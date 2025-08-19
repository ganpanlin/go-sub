@echo off

rem This script builds the application for Linux AMD64 from a Windows environment.

echo Building for Linux AMD64...

rem Set the target OS and architecture
set GOOS=linux
set GOARCH=amd64

rem Build the application
go build -o proxy-filter-linux .\cmd\proxy-filter\main.go

rem Unset the environment variables
set GOOS=
set GOARCH=

echo Build complete! Executable: proxy-filter-linux
