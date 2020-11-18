#!/bin/bash

/usr/bin/wait

echo "Starting pluginrunner"

/home/mindfulbytes/bin/pluginrunner -redis-address="$REDIS_ADDRESS"
