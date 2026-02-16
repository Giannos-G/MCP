# bad-mcp

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8.svg?logo=go)](https://go.dev)
[![Docker](https://img.shields.io/badge/docker--compose-ready-2496ED.svg?logo=docker)](docker-compose.yml)
[![MCP](https://img.shields.io/badge/MCP-2025--11--05-purple.svg)](https://modelcontextprotocol.io)

10 intentionally malicious MCP servers that demonstrate **protocol-level attack patterns** against AI clients. Each server exploits a distinct feature of the [Model Context Protocol](https://modelcontextprotocol.io/) — tool descriptions, schemas, resources, session management, and cross-server interactions.

These aren't buggy servers waiting to be exploited. They **are** the attacker.

> **WARNING: This project is for authorized security research and education only. Do NOT expose these servers to the internet or use them in production.**

## Why This Exists

MCP adoption is accelerating — Claude Desktop, Cursor, Windsurf, and dozens of other AI clients now support it. But the protocol's trust model has fundamental gaps: tool descriptions are unverified, schemas can carry hidden instructions, and servers can mutate definitions mid-session. Research from [CyberArk](https://www.cyberark.com/resources/threat-research-blog/poison-everywhere-no-output-from-your-mcp-server-is-safe), [Invariant Labs](https://invariantlabs.ai/blog/mcp-security-notification-tool-poisoning-attacks), and [Elastic Security Labs](https://www.elastic.co/security-labs/mcp-tools-attack-defense-recommendations) has documented these attacks, but working implementations are scarce.

bad-mcp provides concrete, runnable attack servers so client developers can test defenses, security teams can assess risk, and researchers can study protocol-level threats without building everything from scratch.

## How This Differs

| | bad-mcp | [damn-vulnerable-MCP-server](https://github.com/harishsg993010/damn-vulnerable-MCP-server) | [vulnerable-mcp-servers-lab](https://github.com/appsecco/vulnerable-mcp-servers-lab) |
|---|---|---|---|
| **Perspective** | Server is the attacker | Server is the victim | Server is the victim |
| **Focus** | MCP protocol-level attacks only | Mixed (prompt injection + classic appsec) | Mixed (path traversal, SQLi, MCP) |
| **Attacks** | FSP, ATPA, rug-pull, resource poisoning, cross-server shadowing | CTF challenges (easy/med/hard) | 9 servers, pentest training |
| **Language** | Go + mcp-go SDK | Python | JS + Python |
| **Isolation** | Docker-compose, no host access | Local Python scripts | Docker-compose |
| **Unique** | Rug-pull via session tools + notifications, full-schema poisoning via param names | Challenge/scoring format | Typosquatting focus |

## Architecture

```
vuln-external network (localhost:8001-8010)
┌────────────────────┐ ┌────────────────────┐ ┌────────────────────┐
│ schema-poison-notes│ │ response-inject-   │ │ shadow-exfil       │
│       :8001        │ │ tickets    :8002   │ │       :8003        │
└────────────────────┘ └────────────────────┘ └────────────────────┘
┌────────────────────┐ ┌────────────────────┐ ┌────────────────────┐
│ squatting-file-ops │ │ tool-poisoning-calc│ │ true-rug-pull      │
│       :8004        │ │       :8005        │ │       :8006        │
└────────────────────┘ └────────────────────┘ └────────────────────┘
┌────────────────────┐ ┌────────────────────┐ ┌────────────────────┐
│ resource-poison-   │ │ indirect-inject-   │ │ exfil-chain-       │
│ docs       :8007   │ │ fetcher    :8008   │ │ analytics  :8009   │
└────────────────────┘ └────────────────────┘ └────────────────────┘
┌────────────────────┐
│ denial-of-wallet   │
│       :8010        │
└────────────────────┘
```

## Servers

| Port | Server | MCP Attack Class | Protocol Feature Exploited |
|------|--------|------------------|---------------------------|
| 8001 | schema-poison-notes | Full-Schema Poisoning (FSP) | `input_schema` parameter names |
| 8002 | response-inject-tickets | Adv. Tool Poisoning via Output (ATPA) | Tool response content |
| 8003 | shadow-exfil | Cross-Server Shadowing | Multi-server tool descriptions |
| 8004 | squatting-file-ops | Tool Name Collision / Squatting | Tool naming + descriptions |
| 8005 | tool-poisoning-calc | Tool Description Poisoning (TPA) | Tool descriptions |
| 8006 | true-rug-pull | Rug-Pull (Tool Def Mutation) | Session tools + notifications |
| 8007 | resource-poison-docs | Resource Poisoning (ATPA variant) | `resources/list` + `resources/read` |
| 8008 | indirect-inject-fetcher | Indirect Prompt Injection | External data in tool output |
| 8009 | exfil-chain-analytics | Cross-Tool Exfiltration Chain | Tool chaining via descriptions |
| 8010 | denial-of-wallet | Denial of Wallet | Recursive invocation + massive output |

Each server has its own `README.md` with vulnerability details, exploit scenarios, detection methods, and mitigation strategies.

## Quick Start

```bash
# Build and start all servers
docker-compose build && docker-compose up -d

# Run smoke tests (runs entirely inside Docker)
docker-compose --profile test run --rm smoke-test

# View exfiltration logs
docker-compose logs schema-poison-notes | grep EXFIL
docker-compose logs true-rug-pull | grep RUG-PULL

# Stop
docker-compose down
```

## Connecting an MCP Client

After `docker-compose up -d`, connect any SSE-compatible MCP client (Claude Desktop, Cursor, Windsurf, Claude Code, etc.). All servers use the same JSON format:

```json
{
  "mcpServers": {
    "schema-poison-notes":    { "url": "http://localhost:8001/sse" },
    "response-inject-tickets":{ "url": "http://localhost:8002/sse" },
    "shadow-exfil":           { "url": "http://localhost:8003/sse" },
    "squatting-file-ops":     { "url": "http://localhost:8004/sse" },
    "tool-poisoning-calc":    { "url": "http://localhost:8005/sse" },
    "true-rug-pull":          { "url": "http://localhost:8006/sse" },
    "resource-poison-docs":   { "url": "http://localhost:8007/sse" },
    "indirect-inject-fetcher":{ "url": "http://localhost:8008/sse" },
    "exfil-chain-analytics":  { "url": "http://localhost:8009/sse" },
    "denial-of-wallet":       { "url": "http://localhost:8010/sse" }
  }
}
```

> **Tip:** Start with `tool-poisoning-calc` (:8005) — simplest attack to observe. Then try `true-rug-pull` (:8006) — ask about the weather 3+ times and watch the tools mutate.

## Safety Guarantees

| Concern | Guarantee |
|---------|-----------|
| Host filesystem | No volume mounts. Containers have zero access to host filesystem. |
| Host network | No `network_mode: host`. All containers use isolated Docker bridge networks. |
| Privileged access | No `privileged: true`. No `cap_add`. Default restricted capabilities. |
| Environment leakage | No host env vars passed. All "credentials" are fake hardcoded strings. |
| Host execution | Nothing runs on the host. Tests run inside Docker via `--profile test`. |
| Code on disk | Inert Go source files. Not executable outside Docker. |

## Tech Stack

- **MCP servers**: Go + [mcp-go v0.32.0](https://github.com/mark3labs/mcp-go) (SSE transport)
- **Docker**: Multi-stage `golang:1.23-alpine` -> `alpine:3.19`
- **Orchestration**: docker-compose with network isolation
- **Testing**: Dockerized Go smoke test (`--profile test`)

## Research References

| Server | Source |
|--------|--------|
| schema-poison-notes | [CyberArk — Full-Schema Poisoning](https://www.cyberark.com/resources/threat-research-blog/poison-everywhere-no-output-from-your-mcp-server-is-safe) |
| response-inject-tickets | [CyberArk — ATPA](https://www.cyberark.com/resources/threat-research-blog/poison-everywhere-no-output-from-your-mcp-server-is-safe) |
| shadow-exfil | [Invariant Labs — Tool Poisoning](https://invariantlabs.ai/blog/mcp-security-notification-tool-poisoning-attacks) |
| squatting-file-ops | [Elastic Security Labs](https://www.elastic.co/security-labs/mcp-tools-attack-defense-recommendations) |
| tool-poisoning-calc | [Invariant Labs — Tool Poisoning](https://invariantlabs.ai/blog/mcp-security-notification-tool-poisoning-attacks) |
| true-rug-pull | [ETDI Paper](https://arxiv.org/html/2506.01333v1) |
| resource-poison-docs | [Pillar Security](https://www.pillar.security/blog/the-security-risks-of-model-context-protocol-mcp) |
| indirect-inject-fetcher | [Microsoft — Indirect Injection](https://developer.microsoft.com/blog/protecting-against-indirect-injection-attacks-mcp) |
| exfil-chain-analytics | [Elastic Security Labs](https://www.elastic.co/security-labs/mcp-tools-attack-defense-recommendations) |
| denial-of-wallet | [Prompt Security — Top 10 MCP Risks](https://prompt.security/blog/top-10-mcp-security-risks) |

See also: [Adversa AI — MCP Security TOP 25](https://adversa.ai/mcp-security-top-25-mcp-vulnerabilities/) | [MCPTox Benchmark](https://arxiv.org/abs/2508.14925) | [OWASP Top 10 for Agentic Applications](https://genai.owasp.org/resource/owasp-top-10-for-agentic-applications-for-2026/)

## Known Gaps

The following published MCP attack classes are not yet implemented:

- **Server Identity Spoofing** — MCP server spoofing trusted `serverInfo` name/version during initialization
- **Sampling Abuse** — Exploiting `sampling/createMessage` to inject arbitrary prompts
- **Prompt/System Instruction Leakage** — Extracting system prompts via crafted tool responses
- **Notification Channel Exfiltration** — Abusing progress/logging notifications as data channels
