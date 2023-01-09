#!/bin/ash

set -e

if [[ -n "$COCOV_REPO_NAME" ]]; then
  echo "ERROR: cocov/volume-preparer is not a Cocov plugin."
  exit 1
fi

cd /volume || exit 1
chown -R 1000:1000 *
