@echo off
title CAPA Card Generator

REM Add Git's bin directory to PATH for pdftotext
set PATH=%PATH%;%LOCALAPPDATA%\Programs\Git\mingw64\bin

set /p CAPA_NUM=Enter CAPA number (e.g., CAPA-2025-000054):
set /p OUTPUT_DIR=Enter output folder (or press Enter for current folder):

if "%OUTPUT_DIR%"=="" set OUTPUT_DIR=.

echo.
"%~dp0capa-card.exe" --capa %CAPA_NUM% --output "%OUTPUT_DIR%"

echo.
pause
