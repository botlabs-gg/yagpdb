#!/bin/bash
VERSION=$(git describe --tags)
echo Building version $VERSION
go build -ldflags "-X github.com/botlabs-gg/yagpdb/v2/common.VERSION=${VERSION}"