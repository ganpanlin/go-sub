#!/bin/bash

# This script runs the proxy filter application.
# It passes its arguments to the application's command-line flags.

# USAGE:
# ./run.sh [config_path] [port]
#
# EXAMPLE:
# ./run.sh config.json 8080
# ./run.sh /etc/proxy-filter/config.json 8888

# Set default values
CONFIG_PATH="config.json"
PORT="18080"

# Override defaults with script arguments if provided
if [ -n "$1" ]; then
    CONFIG_PATH=$1
fi

if [ -n "$2" ]; then
    PORT=$2
fi

# The frontend directory is assumed to be in the same location as the script
FRONTEND_PATH="./frontend"

# Run the application
echo "Starting proxy-filter in the background on port $PORT..."
./proxy-filter-linux -config "$CONFIG_PATH" -port "$PORT" -frontend-dir "$FRONTEND_PATH" &
