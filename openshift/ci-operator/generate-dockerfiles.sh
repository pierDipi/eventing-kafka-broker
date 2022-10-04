#!/bin/bash

set -e

function generate_dockefiles() {
  local target_dir=$1; shift
  # Remove old images and re-generate, avoid stale images hanging around.
  for img in $@; do
    local image_base=$(basename $img)
    mkdir -p $target_dir/$image_base
    bin=$image_base envsubst < openshift/ci-operator/Dockerfile.in > $target_dir/$image_base/Dockerfile
  done
}

generate_dockefiles $@

# shellcheck disable=SC2002
go_version=$(grep "^go.*" "go.mod" | awk '{print $2}')

dockerfile_replace="s|registry.ci.openshift.org/openshift/release:golang.*|registry.ci.openshift.org/openshift/release:golang-${go_version}|g"

dockerfiles=$(find openshift/ci-operator/ -name Dockerfile -type f 2> /dev/null)

for f in ${dockerfiles[@]}; do \
  sed -i "${dockerfile_replace}" "${f}"
done
