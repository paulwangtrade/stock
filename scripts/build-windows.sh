#! /bin/bash

echo -e "Start running the script..."
cd "$(dirname "$0")/.."

echo -e "Start building the app for windows platform..."
wails build --platform windows/amd64

echo -e "End running the script!"
