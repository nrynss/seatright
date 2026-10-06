#!/bin/sh
set -eu
unset LD_LIBRARY_PATH LD_PRELOAD
exec /home/nryn/.local/bin/sbx exec -i -w "$PWD" seatright-codex /home/nryn/.npm-global/lib/node_modules/@openai/codex/node_modules/@openai/codex-linux-x64/vendor/x86_64-unknown-linux-musl/bin/codex "$@"
