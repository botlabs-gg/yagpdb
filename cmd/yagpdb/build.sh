#!/bin/bash
VERSION=$(git describe --tags)
echo Building version $VERSION
go build -ldflags "-X github.com/jonas747/yagpdb/common.VERSION=${VERSION}"