#!/bin/ash

set -e

cd /volume || exit 1
filename="$(find /tmp -not -path '*/.*' -type f -name '*.tar' -print -quit)"
tar --strip-components=1 -xf "$filename"
rm "$filename"
chown -R 1000:1000 ./*
