#!/bin/bash

/usr/bin/wait

echo "Starting crawler"

/home/mindfulbytes/bin/crawler -redis-address="$REDIS_ADDRESS"
