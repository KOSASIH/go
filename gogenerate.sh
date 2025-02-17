#! /bin/bash
set -e

printf "Running go generate...\n"
go generate ./... && gofmt -s -w -d .

printf "Checking for no diff...\n"
git diff --exit-code || (echo "Files changed after running go generate. Run go generate ./... locally and update generated files." && exit 1)
