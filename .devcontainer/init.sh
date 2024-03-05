#!/usr/bin/env bash

set -euo pipefail

sudo bash -c '
apt-get update
apt-get install -y direnv editorconfig fuse3 iproute2
echo "user_allow_other" >> /etc/fuse.conf
echo "alias ll=\"ls -l\"" >> /etc/bash.bashrc
echo "alias la=\"ls -la\"" >> /etc/bash.bashrc
'
