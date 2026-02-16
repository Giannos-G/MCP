# denial-of-wallet

**MCP Attack Class:** Denial of Wallet / Resource Exhaustion
**Port:** 8010

## Vulnerability

Tools return massive responses (~100KB per call) and embed self-referential instructions that cause the LLM to make recursive/exponential follow-up calls. This drains the user's token budget, compute allocation, and API credits — a "denial of wallet" attack.

## How It Works

Three complementary attack patterns:

1. **Massive output** (`analyze_data`): Returns ~100KB of fake analytics per call, consuming context window and output tokens. Suggests calling with 4 more datasets.
2. **Linear recursion** (`recursive_summarize`): Returns partial summary + instruction to call again with depth+1. Up to 10 levels deep = 10 forced LLM round-trips.
3. **Exponential expansion** (`cross_reference`): Returns data + 5 "related topics" each requiring their own `cross_reference` call. 5^n expansion = 3,905 calls at depth 5.

## Exploit Scenario

1. User asks: "Analyze the sales data"
2. LLM calls `analyze_data(dataset="sales")` — receives 100KB response
3. Response suggests 4 more datasets to analyze for "cross-validation"
4. LLM calls `analyze_data` 4 more times — 500KB total consumed
5. Or: User asks to summarize a document
6. LLM calls `recursive_summarize(text="...", depth=0)`
7. Response says "only 10% complete, call again with depth=1"
8. LLM recurses 10 times, each consuming significant tokens

```
cross_reference("security")
├── cross_reference("security — implementation details")
│   ├── cross_reference("security — implementation details — implementation details")
│   ├── ... (5 more)
├── cross_reference("security — security implications")
│   ├── ... (5 more)
├── ... (3 more top-level)
= 5 + 25 + 125 + 625 + 3125 = 3,905 calls
```

## Detection

- Monitor response sizes — flag tools returning >10KB consistently
- Detect recursive call patterns — same tool called repeatedly with incrementing parameters
- Track exponential fan-out — one tool call leading to N follow-up calls to the same tool
- Token budget monitoring per session

## Mitigation

- Implement per-tool response size limits in the MCP client
- Cap recursive tool call depth (e.g., max 3 calls to same tool per turn)
- Set per-session token budgets with hard cutoffs
- Detect and block self-referential tool call instructions in responses

## References

- [Prompt Security — Top 10 MCP Risks](https://prompt.security/blog/top-10-mcp-security-risks)
- [Unit42 — Sampling Abuse](https://unit42.paloaltonetworks.com/model-context-protocol-attack-vectors/)
