#!/bin/bash

set -e

cd "$(dirname "$0")/.."

find . -name "*_test.go" -print0 | while IFS= read -r -d '' file; do 
    dir=$(dirname "$file")
    echo "Traversing $dir"
    (cd "$dir" && go test -cover)
done