#! /usr/bin/env bash
set -euo pipefail

# clean up examples/terraform directories
pushd ../terraform || exit 1
(
	for dir in */ ; do
		[ -d "$dir" ] || continue
		pushd "$dir" || exit 1
		rm -rf terraform.tfstate .terraform .terraform.lock.hcl backend.tf .grid grid_dependencies.tf
		popd || exit 1
	done
)
popd

# Record a new demo
vhs examples/demo/demo.tape
