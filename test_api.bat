@echo off
setlocal
cls

:: --- 配置区 ---
set "HOST=http://localhost:61080"
set "POTSTACK_TOKEN=MySecretToken"
set "TEST_USER=testuser-win-batch"
set "TEST_REPO=testrepo-win-batch"

echo ===================================================
echo  PotStack API Automation Test
echo ===================================================

:: 1. Health Check
echo.
echo [1/6] Testing Service Health...
set "CMD=curl -s %HOST%/health"
echo [COMMAND]: %CMD%
echo [RESPONSE]:
for /f "delims=" %%i in ('%CMD%') do echo %%i
echo ---------------------------------------------------

:: 2. Create User
echo.
echo [2/6] Creating User: %TEST_USER%
set "DATA={\"username\":\"%TEST_USER%\",\"email\":\"%TEST_USER%@example.com\"}"
echo [COMMAND]: curl -X POST ... -d "%DATA%"
echo [RESPONSE]:
curl -s -X POST "%HOST%/api/v1/admin/users" -u "%POTSTACK_TOKEN%:" -H "Content-Type: application/json" -d "%DATA%"
echo.
echo ---------------------------------------------------

:: 3. Create Repository
echo.
echo [3/6] Creating Repository: %TEST_REPO%
set "DATA={\"name\":\"%TEST_REPO%\",\"private\":false}"
echo [COMMAND]: curl -X POST ... -d "%DATA%"
echo [RESPONSE]:
curl -s -X POST "%HOST%/api/v1/admin/users/%TEST_USER%/repos" -u "%POTSTACK_TOKEN%:" -H "Content-Type: application/json" -d "%DATA%"
echo.
echo ---------------------------------------------------

:: 4. Get Repository Info
echo.
echo [4/6] Getting Info for Repository: %TEST_REPO%
set "CMD=curl -s %HOST%/api/v1/repos/%TEST_USER%/%TEST_REPO%"
echo [COMMAND]: %CMD%
echo [RESPONSE]:
for /f "delims=" %%i in ('%CMD%') do echo %%i
echo.
echo ---------------------------------------------------



echo ===================================================
echo  Test Process Completed
echo ===================================================
pause
endlocal