@echo off
:: PotStack Windows 启动脚本

:: 1. 基础配置
set POTSTACK_DATA_DIR=data
set POTSTACK_HTTP_PORT=61080

:: 2. 认证令牌 (建议修改)
set POTSTACK_TOKEN=changeme

:: 3. 启动应用
echo Starting PotStack...
echo Dir: %POTSTACK_DATA_DIR%
echo Port: %POTSTACK_HTTP_PORT%

potstack.exe
pause
