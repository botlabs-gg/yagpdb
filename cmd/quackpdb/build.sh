#!/bin/bash
VERSION=$(git describe --tags)
echo Quacking quacksion $VERSION
go build -ldflags "-X github.com/botlabs-gg/quackpdb/v2/common.VERSION=${VERSION}"