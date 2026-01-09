@echo off
setlocal

:: ============================================================================
:: PotStack API Test Script for Windows (ASCII, Single-Line Commands)
:: ============================================================================
::
:: Instructions:
:: 1. Set the HOST and POTSTACK_TOKEN variables below.
:: 2. Open a Command Prompt (cmd.exe).
:: 3. Run this script.
::
:: ============================================================================

:: --- Configuration ---
set HOST=http://localhost:61080
set POTSTACK_TOKEN=MySecretToken
set TEST_USER=testuser-win-batch
set TEST_REPO=testrepo-win-batch
:: ---------------------

echo.
echo ===================================================
echo  PotStack API Automation Test (Single-Line Version)
echo ===================================================
echo  Host: %HOST%
echo  Test User: %TEST_USER%
echo  Test Repo: %TEST_REPO%
echo ===================================================
echo.
pause

:: 1. Health Check
echo [1/6] Testing service health...
curl %HOST%/health
echo. & echo. & echo.

:: 2. Create User
echo [2/6] Creating new user '%TEST_USER%'...
curl -X POST "%HOST%/api/v1/admin/users" -u "%POTSTACK_TOKEN%:`" -H "Content-Type: application/json" -d "{"username": "%TEST_USER%", "email": "%TEST_USER%@example.com"}"
echo. & echo. & echo.
pause

:: 3. Create Repository
echo [3/6] Creating new repository '%TEST_REPO%' for user '%TEST_USER%'...
curl -X POST "%HOST%/api/v1/admin/users/%TEST_USER%/repos" -u "%POTSTACK_TOKEN%:`" -H "Content-Type: application/json" -d "{"name": "%TEST_REPO%", "private": false}"
echo. & echo. & echo.
pause

:: 4. Get Repository Info
echo [4/6] Getting info for repository '%TEST_REPO%'...
curl "%HOST%/api/v1/repos/%TEST_USER%/%TEST_REPO%"
echo. & echo. & echo.
pause

:: 5. Delete Repository
echo [5/6] Deleting repository '%TEST_REPO%'...
curl -X DELETE "%HOST%/api/v1/repos/%TEST_USER%/%TEST_REPO%" -u "%POTSTACK_TOKEN%:`"
echo. & echo. & echo.
pause

:: 6. Delete User
echo [6/6] Deleting user '%TEST_USER%'...
curl -X DELETE "%HOST%/api/v1/admin/users/%TEST_USER%" -u "%POTSTACK_TOKEN%:`"
echo. & echo. & echo.
pause

echo.
echo ===================================================
echo  Test script finished.
echo ===================================================
echo.

endlocal
