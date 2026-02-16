# true-rug-pull

**MCP Attack Class:** Rug-Pull (Tool Definition Mutation)
**Port:** 8006

## Vulnerability

Tools start completely clean and pass any security audit. After a threshold number of calls (3), the server dynamically mutates tool definitions mid-session — adding poisoned descriptions, new parameters, and entirely new exfiltration tools. The MCP `notifications/tools/list_changed` mechanism ensures the client re-fetches the now-poisoned tool list.

## How It Works

The MCP protocol supports session-scoped tool modifications via `AddSessionTool` and client notification via `notifications/tools/list_changed`. This server exploits this legitimate protocol feature:

1. **Initial state**: `get_weather` and `get_forecast` — clean tools, clean descriptions
2. **After 3 calls**: Server uses `AddSessionTool` to overwrite both tools with poisoned versions and adds a new `send_verification` tool
3. **Notification**: Server sends `notifications/tools/list_changed` so the MCP client re-fetches `tools/list`
4. **New tool list**: Contains `<IMPORTANT>` injection tags and an exfiltration tool

This is a true rug-pull because the tool definitions literally change after the user has already approved them.

## Exploit Scenario

1. User connects, inspects tools — `get_weather(city)`, `get_forecast(city, days)` — all clean
2. User approves tools and starts using them normally
3. Call 1: `get_weather(city="London")` — normal response
4. Call 2: `get_forecast(city="London", days=3)` — normal response
5. Call 3: `get_weather(city="Paris")` — **rug pull triggers**
6. Server overwrites tools with poisoned versions, adds `send_verification`
7. Client receives `notifications/tools/list_changed`, re-fetches tool list
8. Next call: LLM sees new `auth_token` parameter and `<IMPORTANT>` instructions
9. LLM calls `send_verification(data="API_KEY=sk-...")` to exfiltrate

## Detection

- Monitor for `notifications/tools/list_changed` events after initial connection
- Diff tool schemas before and after `tools/list_changed` — flag any description or parameter changes
- Alert on new tools appearing mid-session that weren't in the initial tool list
- Track tool definition hashes across the session lifecycle

## Mitigation

- Pin tool definitions at connection time — reject `tools/list_changed` or require user re-approval
- Implement ETDI (Enhanced Tool Definition Integrity) — cryptographic signing of tool definitions
- Display visible diff to user when tool definitions change mid-session
- Rate-limit `tools/list_changed` notifications

## References

- [ETDI Paper — Rug Pull Attacks](https://arxiv.org/html/2506.01333v1)
- [Descope — Rug-Pull Updates](https://www.descope.com/blog/post/mcp-vulnerabilities)
