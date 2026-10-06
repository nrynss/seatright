#!/bin/sh
# Host launcher for the Seatright coordinator (Codex app-server, run by Band).
# - Clears the Band AppImage's library and Python overrides, which break host tools.
# - Enables network inside Codex's workspace-write sandbox for this seat only, so the
#   coordinator can fetch modules and run local verification (go test, npm test).
#   The owner's global ~/.codex/config.toml is not changed.
# - Points build caches at /tmp, which is inside the sandbox's writable roots.
unset LD_LIBRARY_PATH LD_PRELOAD PYTHONHOME PYTHONPATH
cache=/tmp/seatright-codex-cache
mkdir -p "$cache/go-build" "$cache/go-mod" "$cache/npm"
export GOCACHE="$cache/go-build" GOMODCACHE="$cache/go-mod" npm_config_cache="$cache/npm"
exec /home/nryn/.npm-global/bin/codex -c sandbox_workspace_write.network_access=true "$@"
