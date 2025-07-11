#!/bin/bash
VERSION=$(git describe --tags)
echo Building version $VERSION
go build -ldflags "-X github.com/RhykerWells/yagpdb/v2/common.VERSION=${VERSION}"