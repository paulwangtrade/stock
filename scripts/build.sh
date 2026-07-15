#! /bin/bash

echo -e "Start running the script..."
cd "$(dirname "$0")/.."

echo -e "Start building the app..."
wails build

echo -e "End running the script!"
