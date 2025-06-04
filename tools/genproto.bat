@echo off
setlocal

set PROTO_ROOT=%~dp0..

echo %PROTO_ROOT%

rem Generate for infra/protocol
protoc.exe ^
    --go_out=%PROTO_ROOT%\infra\pb\protocol ^
    --go-grpc_out=%PROTO_ROOT%\infra\pb\protocol ^
    --go_opt=module=github.com/phuhao00/dafuweng/infra/pb/protocol ^
    --go-grpc_opt=module=github.com/phuhao00/dafuweng/infra/pb/protocol ^
    --proto_path=%PROTO_ROOT% ^
    --proto_path=%PROTO_ROOT%\infra\protocol ^
    %PROTO_ROOT%\infra\protocol\*.proto

rem Generate for infra/model
protoc.exe ^
    --go_out=%PROTO_ROOT%\infra\pb\model ^
    --go-grpc_out=%PROTO_ROOT%\infra\pb\model ^
    --go_opt=module=github.com/phuhao00/dafuweng/infra/pb/model ^
    --go-grpc_opt=module=github.com/phuhao00/dafuweng/infra/pb/model ^
    --proto_path=%PROTO_ROOT% ^
    --proto_path=%PROTO_ROOT%\infra\model ^
    --proto_path=%PROTO_ROOT%\infra\protocol ^
    %PROTO_ROOT%\infra\model\*.proto

rem Generate self define tag

for /r "%PROTO_ROOT%\infra\pb\model" %%f in (*.pb.go) do (
    protoc-go-inject-tag -input="%%f"
)

for /r "%PROTO_ROOT%\infra\pb\protocol" %%f in (*.pb.go) do (
    protoc-go-inject-tag -input="%%f"
)

endlocal