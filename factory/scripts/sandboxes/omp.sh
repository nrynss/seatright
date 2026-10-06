#!/bin/sh
set -eu
unset LD_LIBRARY_PATH LD_PRELOAD PYTHONHOME PYTHONPATH
# jamd may spawn this launcher from its AppImage mount; that path does not exist in the VM.
wd=$(pwd -P)
case "$wd" in /tmp/.mount_*) wd=/home/nryn/work/seatright ;; esac
# Band offers its host MCP server to ACP seats; the sandbox cannot spawn it, so drop it.
exec /usr/bin/python3 /home/nryn/work/seatright/scripts/sandboxes/acp-strip-mcp.py \
  /home/nryn/.local/bin/sbx exec -i -w "$wd" seatright-omp /opt/seatright/omp acp "$@"
