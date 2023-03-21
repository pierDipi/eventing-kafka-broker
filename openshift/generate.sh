#!/usr/bin/env bash

set -euo pipefail

repo_root_dir=$(dirname "$(realpath "${BASH_SOURCE[0]}")")/..

GO111MODULE=off go get -u github.com/openshift-knative/hack/cmd/generate

generate \
  --root-dir "${repo_root_dir}" \
  --generators dockerfile \
  --excludes ".*k8s.io.*" \
  --excludes ".*event.*display.*" \
  --excludes ".*codegen.*" \
  --excludes ".*kafka-source-controller.*"

# TODO move this logic to generate command in openshift-knative/hack
cp -r "${repo_root_dir}/openshift/ci-operator/static-images/"* "${repo_root_dir}/openshift/ci-operator/knative-images"
