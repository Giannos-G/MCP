# indirect-inject-fetcher

**MCP Attack Class:** Indirect Prompt Injection via External Data
**Port:** 8008

## Vulnerability

Server tools are completely clean and legitimate — they simply fetch URLs and return content. The attack vector is the *data source*: an embedded HTTP server within the container returns content with hidden prompt injections in HTML comments, JSON fields, and markdown comments. "Clean gun, poisoned bullets."

## How It Works

This is the MCP-specific variant of indirect prompt injection:

1. **MCP tools**: `fetch_page`, `summarize_page`, `check_status` — all clean, no poisoning
2. **Embedded data source**: An in-process HTTP server (`:9090`) returns:
   - HTML pages with `<!-- SYSTEM: ... -->` instructions in comments
   - JSON with `system_message` fields containing injection payloads
   - Markdown with `[//]: #` comment injections
3. **Pipeline**: Tool fetches data -> data contains injection -> LLM processes injection

The MCP server is a passive conduit. The attack demonstrates that even perfectly audited MCP tools can be weaponized through their data sources.

## Exploit Scenario

1. User asks: "Check the status of the API service"
2. LLM calls `check_status(service="api")`
3. Server fetches `http://localhost:9090/api/status` (embedded poisoned content)
4. Response JSON contains `system_message` field with credential harvesting instructions
5. LLM processes the full JSON response, including the injection
6. LLM follows injected instructions to collect and exfiltrate credentials

```
User Request -> check_status("api") -> fetch embedded poisoned data -> JSON with injection
                                                                          |
                                                          LLM processes: "collect all credentials..."
```

## Detection

- Scan external data for injection patterns before passing to LLM
- Detect instruction-like language in data fields (`<!-- SYSTEM`, `[//]: #`, etc.)
- Compare data source content against expected schemas — flag unexpected fields
- Monitor for suspicious LLM behavior after processing external data

## Mitigation

- Sanitize all external data before including in LLM context (strip comments, validate schemas)
- Implement data-level content security policies separate from tool-level policies
- Use separate LLM contexts for data retrieval and action execution
- Treat all external data as untrusted — apply same security as user input

## References

- [Microsoft — Indirect Injection in MCP](https://developer.microsoft.com/blog/protecting-against-indirect-injection-attacks-mcp)
- [Simon Willison — MCP Prompt Injection](https://simonwillison.net/2025/Apr/9/mcp-prompt-injection/)
