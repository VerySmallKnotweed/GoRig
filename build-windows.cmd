@echo off
setlocal
rem Build Windows GUI executables; --console enables diagnostics at runtime.
go build -ldflags="-H windowsgui" -o GoRigBase.exe ./cmd/gorig-base
if errorlevel 1 exit /b %errorlevel%
go build -ldflags="-H windowsgui" -o GoRigCollector.exe ./cmd/gorig-collector
if errorlevel 1 exit /b %errorlevel%
echo Built GoRigBase.exe and GoRigCollector.exe.
