#!/bin/sh
set -eu
unset LD_LIBRARY_PATH LD_PRELOAD
# jamd may spawn this launcher from its AppImage mount; that path does not exist in the VM.
wd=$(pwd -P)
case "$wd" in /tmp/.mount_*) wd=/home/nryn/work/seatright ;; esac
exec /home/nryn/.local/bin/sbx exec -i -w "$wd" seatright-zcode /home/nryn/work/seatright/.tools/zcode-acp/start-band.sh "$@"
