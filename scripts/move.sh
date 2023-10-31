#!/bin/bash
# move updated binary to /usr/local/bin

set -e

if [[ $EUID -ne 0 ]]; then
   echo "Root privileges required"
   exit 1
fi

mv "$1" /usr/local/bin/knoxctl 
chmod 755 /usr/local/bin/knoxctl

echo "knoxctl moved to /usr/local/bin/knoxctl"