#!/bin/bash

cp meta.yaml ${PLUGIN_DEST}/meta.yaml
go build main.go
cp main ${PLUGIN_DEST}/main
