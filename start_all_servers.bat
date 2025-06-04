@echo off
REM Script to start all servers for the Dafuweng project.
REM It is recommended to run this script from the project root directory.

title Start All Dafuweng Servers

REM Create logs directory if it doesn't exist
if not exist logs mkdir logs
echo Logs directory ensured.

echo Starting servers...
echo --------------------

REM Server details: Directory, Main Go File, Server Name (for title/log)
REM Format: START "Window Title" /MIN go run cmd\<dir>\<mainfile>.go > logs\<logname>.log 2>&1

echo Starting loginserver...
START "Dafuweng - Login Server" /MIN go run cmd\loginserver\loginserver.go > logs\loginserver.log 2>&1
echo   Logs: logs\loginserver.log

echo Starting friendserver...
START "Dafuweng - Friend Server" /MIN go run cmd\friendserver\friendserver.go > logs\friendserver.log 2>&1
echo   Logs: logs\friendserver.log

echo Starting payserver...
START "Dafuweng - Pay Server" /MIN go run cmd\payserver\payserver.go > logs\payserver.log 2>&1
echo   Logs: logs\payserver.log

echo Starting roomserver...
START "Dafuweng - Room Server" /MIN go run cmd\roomserver\roomserver.go > logs\roomserver.log 2>&1
echo   Logs: logs\roomserver.log

echo Starting gameserver...
START "Dafuweng - Game Server" /MIN go run cmd\gameserver\gameserver.go > logs\gameserver.log 2>&1
echo   Logs: logs\gameserver.log

echo Starting gatewayserver...
REM Note: The main file for gatewayserver is gateway.go
START "Dafuweng - Gateway Server" /MIN go run cmd\gatewayserver\gateway.go > logs\gatewayserver.log 2>&1
echo   Logs: logs\gatewayserver.log

echo Starting gmserver...
START "Dafuweng - GM Server" /MIN go run cmd\gmserver\gmserver.go > logs\gmserver.log 2>&1
echo   Logs: logs\gmserver.log

echo --------------------
echo All server processes have been initiated.
echo Each server is running in a separate (minimized) window and logging to its file in the 'logs' directory.
echo Please check the individual log files for startup errors or messages.
echo.
echo To stop the servers, you can close their respective console windows.
echo Alternatively, if many 'go.exe' processes were started, you might need to use Task Manager
echo or a command like 'taskkill /F /IM go.exe' (this will stop ALL go.exe processes).
echo.
pause
