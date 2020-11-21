#!/bin/bash

/usr/bin/wait

echo "Starting web"

/home/mindfulbytes/bin/restapi -redis-address="$REDIS_ADDRESS" -base-url="$BASE_URL"
