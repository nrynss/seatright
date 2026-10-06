#!/bin/sh
set -eu
unset LD_LIBRARY_PATH LD_PRELOAD PYTHONHOME PYTHONPATH
# jamd may spawn this launcher from its AppImage mount; that path does not exist in the VM.
wd=$(pwd -P)
case "$wd" in /tmp/.mount_*) wd=/home/nryn/work/seatright ;; esac
# Docker Sandboxes sets a placeholder XAI_API_KEY in every VM; Grok prefers it over the
# grok.com login in ~/.grok/auth.json, so drop it. Band offers its host MCP server to ACP
# seats; the sandbox cannot spawn it, so the proxy strips it from session requests.
exec /usr/bin/python3 /home/nryn/work/seatright/scripts/sandboxes/acp-strip-mcp.py \
  /home/nryn/.local/bin/sbx exec -i -w "$wd" seatright-grok \
  env -u XAI_API_KEY -u SBX_CRED_XAI_MODE \
  /opt/seatright/grok agent --always-approve -m grok-4.7 stdio "$@"
