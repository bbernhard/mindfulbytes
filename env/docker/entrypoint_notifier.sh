#!/bin/bash

/usr/bin/wait

/home/mindfulbytes/bin/notifier -redis-address="$REDIS_ADDRESS"
