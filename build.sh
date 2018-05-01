#!/bin/bash

set -euf -o pipefail

echo "GOPATH=$GOPATH"
COCKROACH_BUILDER=$GOPATH/src/github.com/cockroachdb/cockroach/build/builder.sh
if [ -f $COCKROACH_BUILDER ]; then
  image=$(grep 'image=' $COCKROACH_BUILDER | cut -f 2 -d '=')
  version=$(grep 'version=' $COCKROACH_BUILDER | cut -f 2 -d '=')
    echo "Using Docker $image:$version"
else
  echo "CockroachDB repo not found."
  exit 1
fi

DOCKER_RUN="docker run -i -u $(id -u):$(id -g) --rm \
  --volume=$GOPATH/bin:/go/bin \
  --volume=$GOPATH/src/github.com/cockroachdb:/go/src/github.com/cockroachdb \
  $image:$version"

echo "Building roachprod"
$DOCKER_RUN go install github.com/cockroachdb/roachprod

echo "Building roachtest"
$DOCKER_RUN go install github.com/cockroachdb/cockroach/pkg/cmd/roachtest

echo "Building workload"
$DOCKER_RUN go install github.com/cockroachdb/cockroach/pkg/cmd/workload
echo "Done"
