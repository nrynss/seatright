#!/usr/bin/env python3
"""Run an ACP agent command, removing client-offered MCP servers from session requests.

Band offers its own MCP server to every ACP seat as a host command path. A seat running
in a Docker sandbox cannot spawn that host path, and some agents (OMP) fail the whole
session when an offered server does not start. Seats reply through their turn output, so
the offered servers are dropped: `mcpServers` becomes [] in session/new, session/load,
session/resume and session/fork. Every other byte passes through unchanged.

Usage: acp-strip-mcp.py <command> [args...]
"""
import json
import subprocess
import sys
import threading

SESSION_METHODS = {"session/new", "session/load", "session/resume", "session/fork"}


def strip(line: bytes) -> bytes:
    try:
        msg = json.loads(line)
    except ValueError:
        return line
    if isinstance(msg, dict) and msg.get("method") in SESSION_METHODS:
        params = msg.get("params")
        if isinstance(params, dict) and params.get("mcpServers"):
            params["mcpServers"] = []
            return (json.dumps(msg, separators=(",", ":")) + "\n").encode()
    return line


def pump_out(src, dst):
    for chunk in iter(lambda: src.read1(65536), b""):
        dst.write(chunk)
        dst.flush()


def main() -> int:
    child = subprocess.Popen(sys.argv[1:], stdin=subprocess.PIPE, stdout=subprocess.PIPE)
    out = threading.Thread(target=pump_out, args=(child.stdout, sys.stdout.buffer), daemon=True)
    out.start()
    try:
        for line in sys.stdin.buffer:
            child.stdin.write(strip(line))
            child.stdin.flush()
    except (BrokenPipeError, KeyboardInterrupt):
        pass
    finally:
        try:
            child.stdin.close()
        except BrokenPipeError:
            pass
    code = child.wait()
    out.join(timeout=5)
    return code


if __name__ == "__main__":
    sys.exit(main())
