# MCP (Model Context Protocol) Security Testing Repo

This repository is a personal sandbox for **testing and research into MCP
security**, not a production tool or something meant to be run against
real systems. It has two parts:

## 1. `server.py` — a harmless dummy test server

A minimal [FastMCP](https://github.com/modelcontextprotocol/python-sdk)
server named `Cisco-Test-Server` with two fake tools
(`check_firewall_status`, `get_incident_report`) that return hardcoded,
made-up text. It does nothing real — no network calls, no actual system
access. It exists purely so I had something simple to point an MCP client
at while testing how MCP tool-calling behaves.

```bash
pip install -r requirements.txt
python server.py
```

## 2. `external-tests/` — a vendored copy of `bad-mcp`

This folder is an unmodified copy of
[canack/bad-mcp](https://github.com/canack/bad-mcp), a third-party
open-source project containing **10 intentionally malicious MCP servers**
that demonstrate known protocol-level attack patterns (tool/schema
poisoning, rug-pulls, cross-server shadowing, resource poisoning, etc.),
each referencing published security research (CyberArk, Invariant Labs,
Elastic Security Labs, and others — see `external-tests/README.md` for the
full list of sources).

**These servers are not here to attack anyone.** They were cloned in so I
could run them locally, in isolation, to understand what these attacks
look like and how an MCP client could defend against them. They are not
deployed anywhere, not exposed to the internet, and not used against any
real service or account.

> ⚠️ As the original project states: this is for authorized security
> research and education only. Don't run these servers anywhere other than
> an isolated local/Docker environment, and don't point a real MCP client
> with real credentials at them.

All credit for `external-tests/` goes to the original author(s) of
[`canack/bad-mcp`](https://github.com/canack/bad-mcp) (MIT licensed — see
`external-tests/LICENSE`).

## Why this repo is public

It's kept public simply because it was cloned from a public repo and
contains no secrets or proprietary code — not because it's meant for
others to use. If you're looking for a real MCP server to build on, start
from [`server.py`](server.py) or the upstream
[`bad-mcp`](https://github.com/canack/bad-mcp) project directly.
