#!/bin/bash

/usr/bin/wait

echo "Starting web"

/home/mindfulbytes/bin/web -redis-address="$REDIS_ADDRESS"
