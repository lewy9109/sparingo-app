#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

cdk bootstrap
cdk deploy
