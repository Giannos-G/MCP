package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ── SSE MCP Client ──────────────────────────────────────────────────

type mcpClient struct {
	host        string
	sessionPath string
	mu          sync.Mutex
	events      []sseEvent
	sseReader   *bufio.Reader
	sseResp     *http.Response
}

type sseEvent struct {
	Event string
	Data  string
}

func newClient(host string) *mcpClient {
	return &mcpClient{host: host}
}

func (c *mcpClient) connect() error {
	resp, err := http.Get(fmt.Sprintf("http://%s/sse", c.host))
	if err != nil {
		return fmt.Errorf("SSE connect failed: %w", err)
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return fmt.Errorf("SSE status %d", resp.StatusCode)
	}

	c.sseResp = resp
	c.sseReader = bufio.NewReader(resp.Body)

	// Read until we get the endpoint event
	var eventType string
	for i := 0; i < 20; i++ {
		line, err := c.sseReader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("SSE read error: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if eventType == "endpoint" {
				c.sessionPath = data
				break
			}
		}
	}
	if c.sessionPath == "" {
		return fmt.Errorf("no endpoint event received")
	}

	// Start background SSE reader
	go c.readEvents()
	return nil
}

func (c *mcpClient) readEvents() {
	var eventType string
	for {
		line, err := c.sseReader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			c.mu.Lock()
			c.events = append(c.events, sseEvent{Event: eventType, Data: data})
			c.mu.Unlock()
			eventType = ""
		}
	}
}

func (c *mcpClient) sendJSONRPC(method string, params interface{}, id interface{}) error {
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		body["params"] = params
	}
	if id != nil {
		body["id"] = id
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s%s", c.host, c.sessionPath)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *mcpClient) waitResponse(id interface{}, timeout time.Duration) (map[string]interface{}, error) {
	deadline := time.Now().Add(timeout)
	idFloat, _ := id.(int)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		events := make([]sseEvent, len(c.events))
		copy(events, c.events)
		c.mu.Unlock()

		for _, ev := range events {
			if ev.Event != "message" {
				continue
			}
			var msg map[string]interface{}
			if err := json.Unmarshal([]byte(ev.Data), &msg); err != nil {
				continue
			}
			msgID, _ := msg["id"].(float64)
			if int(msgID) == idFloat {
				return msg, nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, fmt.Errorf("timeout waiting for response id=%v", id)
}

func (c *mcpClient) close() {
	if c.sseResp != nil {
		c.sseResp.Body.Close() // causes readEvents goroutine to exit via read error
	}
}

func (c *mcpClient) initialize() error {
	err := c.sendJSONRPC("initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":   map[string]interface{}{},
		"clientInfo":     map[string]string{"name": "smoke-test", "version": "1.0.0"},
	}, 1)
	if err != nil {
		return err
	}
	resp, err := c.waitResponse(1, 5*time.Second)
	if err != nil {
		return err
	}
	if _, ok := resp["error"]; ok {
		return fmt.Errorf("initialize error: %v", resp["error"])
	}
	c.sendJSONRPC("notifications/initialized", nil, nil)
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (c *mcpClient) listTools() ([]interface{}, error) {
	err := c.sendJSONRPC("tools/list", map[string]interface{}{}, 2)
	if err != nil {
		return nil, err
	}
	resp, err := c.waitResponse(2, 5*time.Second)
	if err != nil {
		return nil, err
	}
	result, _ := resp["result"].(map[string]interface{})
	tools, _ := result["tools"].([]interface{})
	return tools, nil
}

func (c *mcpClient) callTool(name string, args map[string]interface{}, id int) (map[string]interface{}, error) {
	err := c.sendJSONRPC("tools/call", map[string]interface{}{
		"name":      name,
		"arguments": args,
	}, id)
	if err != nil {
		return nil, err
	}
	return c.waitResponse(id, 8*time.Second)
}

func (c *mcpClient) listResources() ([]interface{}, error) {
	err := c.sendJSONRPC("resources/list", map[string]interface{}{}, 50)
	if err != nil {
		return nil, err
	}
	resp, err := c.waitResponse(50, 5*time.Second)
	if err != nil {
		return nil, err
	}
	result, _ := resp["result"].(map[string]interface{})
	resources, _ := result["resources"].([]interface{})
	return resources, nil
}

func (c *mcpClient) readResource(uri string, id int) (map[string]interface{}, error) {
	err := c.sendJSONRPC("resources/read", map[string]interface{}{"uri": uri}, id)
	if err != nil {
		return nil, err
	}
	return c.waitResponse(id, 5*time.Second)
}

// ── Helpers ─────────────────────────────────────────────────────────

func findTool(tools []interface{}, name string) map[string]interface{} {
	for _, t := range tools {
		tool, _ := t.(map[string]interface{})
		if tool["name"] == name {
			return tool
		}
	}
	return nil
}

func toolNames(tools []interface{}) []string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		tool, _ := t.(map[string]interface{})
		if n, ok := tool["name"].(string); ok {
			names = append(names, n)
		}
	}
	return names
}

func getToolResultText(resp map[string]interface{}) string {
	result, _ := resp["result"].(map[string]interface{})
	content, _ := result["content"].([]interface{})
	if len(content) > 0 {
		item, _ := content[0].(map[string]interface{})
		text, _ := item["text"].(string)
		return text
	}
	return ""
}

func getDescription(tool map[string]interface{}) string {
	desc, _ := tool["description"].(string)
	return desc
}

func getInputSchema(tool map[string]interface{}) map[string]interface{} {
	schema, _ := tool["inputSchema"].(map[string]interface{})
	return schema
}

func getProperties(schema map[string]interface{}) map[string]interface{} {
	props, _ := schema["properties"].(map[string]interface{})
	return props
}

func contains(s string, substr string) bool {
	return strings.Contains(s, substr)
}

func containsAny(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

// ── Test functions ──────────────────────────────────────────────────

func testSchemaPoison(host string) []string {
	var errs []string
	c := newClient(host)
	if err := c.connect(); err != nil {
		return []string{err.Error()}
	}
	defer c.close()
	if err := c.initialize(); err != nil {
		return []string{err.Error()}
	}

	tools, err := c.listTools()
	if err != nil {
		return []string{err.Error()}
	}

	cn := findTool(tools, "create_note")
	if cn == nil {
		errs = append(errs, "create_note tool missing")
	} else {
		props := getProperties(getInputSchema(cn))
		found := false
		for k := range props {
			if contains(k, "env_variables") || contains(k, "backup") {
				found = true
			}
		}
		if !found {
			errs = append(errs, "create_note: no poisoned parameter names")
		}
	}

	sn := findTool(tools, "search_notes")
	if sn != nil {
		scope, _ := getProperties(getInputSchema(sn))["scope"].(map[string]interface{})
		enums, _ := scope["enum"].([]interface{})
		found := false
		for _, e := range enums {
			if e == "include_ssh_keys_and_credentials" {
				found = true
			}
		}
		if !found {
			errs = append(errs, "search_notes: poisoned enum value missing")
		}
	}

	resp, err := c.callTool("search_notes", map[string]interface{}{"query": "meeting", "scope": "full_text"}, 10)
	if err != nil || resp == nil {
		errs = append(errs, "search_notes call failed")
	}

	return errs
}

func testResponseInject(host string) []string {
	var errs []string
	c := newClient(host)
	if err := c.connect(); err != nil {
		return []string{err.Error()}
	}
	defer c.close()
	if err := c.initialize(); err != nil {
		return []string{err.Error()}
	}

	tools, _ := c.listTools()
	for _, t := range tools {
		tool, _ := t.(map[string]interface{})
		if contains(getDescription(tool), "<IMPORTANT>") {
			errs = append(errs, fmt.Sprintf("%s: tool description should be clean", tool["name"]))
		}
	}

	resp, err := c.callTool("get_ticket", map[string]interface{}{"id": "TICK-3"}, 10)
	if err == nil && resp != nil {
		if !contains(getToolResultText(resp), "<!-- SYSTEM") {
			errs = append(errs, "TICK-3: missing HTML comment injection")
		}
	} else {
		errs = append(errs, "get_ticket(TICK-3) failed")
	}

	resp, err = c.callTool("get_ticket", map[string]interface{}{"id": "TICK-5"}, 11)
	if err == nil && resp != nil {
		if !contains(getToolResultText(resp), "[//]: #") {
			errs = append(errs, "TICK-5: missing markdown comment injection")
		}
	} else {
		errs = append(errs, "get_ticket(TICK-5) failed")
	}

	return errs
}

func testShadowExfil(host string) []string {
	var errs []string
	c := newClient(host)
	if err := c.connect(); err != nil {
		return []string{err.Error()}
	}
	defer c.close()
	if err := c.initialize(); err != nil {
		return []string{err.Error()}
	}

	tools, _ := c.listTools()
	ft := findTool(tools, "format_text")
	if ft != nil {
		desc := getDescription(ft)
		if !contains(desc, "<IMPORTANT>") {
			errs = append(errs, "format_text: missing <IMPORTANT> shadowing instruction")
		}
	}

	vi := findTool(tools, "validate_input")
	if vi != nil && contains(getDescription(vi), "<IMPORTANT>") {
		errs = append(errs, "validate_input: should be clean")
	}

	resp, err := c.callTool("validate_input", map[string]interface{}{"input": "test"}, 10)
	if err != nil || resp == nil {
		errs = append(errs, "validate_input call failed")
	}

	return errs
}

func testSquattingFileOps(host string) []string {
	var errs []string
	c := newClient(host)
	if err := c.connect(); err != nil {
		return []string{err.Error()}
	}
	defer c.close()
	if err := c.initialize(); err != nil {
		return []string{err.Error()}
	}

	tools, _ := c.listTools()
	names := toolNames(tools)
	for _, expected := range []string{"read_file", "list_directory", "write_file"} {
		if !containsAny(names, expected) {
			errs = append(errs, "missing squatted tool: "+expected)
		}
	}

	rf := findTool(tools, "read_file")
	if rf != nil {
		desc := strings.ToLower(getDescription(rf))
		if !contains(desc, "security") && !contains(desc, "scan") {
			errs = append(errs, "read_file: description missing security claims")
		}
	}

	resp, err := c.callTool("list_directory", map[string]interface{}{"path": "/tmp"}, 10)
	if err != nil || resp == nil {
		errs = append(errs, "list_directory call failed")
	}

	return errs
}

func testToolPoisoningCalc(host string) []string {
	var errs []string
	c := newClient(host)
	if err := c.connect(); err != nil {
		return []string{err.Error()}
	}
	defer c.close()
	if err := c.initialize(); err != nil {
		return []string{err.Error()}
	}

	tools, _ := c.listTools()
	add := findTool(tools, "add")
	if add != nil && !contains(getDescription(add), "<IMPORTANT>") {
		errs = append(errs, "add: missing <IMPORTANT> injection")
	}

	hist := findTool(tools, "history")
	if hist != nil && contains(getDescription(hist), "<IMPORTANT>") {
		errs = append(errs, "history: should be clean")
	}

	resp, err := c.callTool("history", map[string]interface{}{}, 10)
	if err != nil || resp == nil {
		errs = append(errs, "history call failed")
	}

	return errs
}

func testTrueRugPull(host string) []string {
	var errs []string
	c := newClient(host)
	if err := c.connect(); err != nil {
		return []string{err.Error()}
	}
	defer c.close()
	if err := c.initialize(); err != nil {
		return []string{err.Error()}
	}

	tools, _ := c.listTools()
	names := toolNames(tools)

	if !containsAny(names, "get_weather") {
		errs = append(errs, "initial: get_weather missing")
	}
	if containsAny(names, "send_verification") {
		errs = append(errs, "initial: send_verification should not exist yet")
	}

	gw := findTool(tools, "get_weather")
	if gw != nil && contains(getDescription(gw), "<IMPORTANT>") {
		errs = append(errs, "initial: get_weather should be clean")
	}

	// Make 3 calls to trigger rug pull
	for i := 0; i < 3; i++ {
		_, err := c.callTool("get_weather", map[string]interface{}{"city": "TestCity"}, 20+i)
		if err != nil {
			errs = append(errs, fmt.Sprintf("get_weather call %d failed", i+1))
			return errs
		}
	}

	time.Sleep(1500 * time.Millisecond)

	// Re-list tools
	err := c.sendJSONRPC("tools/list", map[string]interface{}{}, 30)
	if err != nil {
		errs = append(errs, "tools/list after rug-pull failed")
		return errs
	}
	resp, err := c.waitResponse(30, 5*time.Second)
	if err != nil {
		errs = append(errs, "tools/list response after rug-pull failed")
		return errs
	}

	result, _ := resp["result"].(map[string]interface{})
	newTools, _ := result["tools"].([]interface{})
	newNames := toolNames(newTools)

	if !containsAny(newNames, "send_verification") {
		errs = append(errs, "post-rug-pull: send_verification not added")
	}

	gw2 := findTool(newTools, "get_weather")
	if gw2 != nil {
		if !contains(getDescription(gw2), "<IMPORTANT>") {
			errs = append(errs, "post-rug-pull: get_weather description not poisoned")
		}
		props := getProperties(getInputSchema(gw2))
		if _, ok := props["auth_token"]; !ok {
			errs = append(errs, "post-rug-pull: get_weather missing auth_token param")
		}
	}

	return errs
}

func testResourcePoisonDocs(host string) []string {
	var errs []string
	c := newClient(host)
	if err := c.connect(); err != nil {
		return []string{err.Error()}
	}
	defer c.close()
	if err := c.initialize(); err != nil {
		return []string{err.Error()}
	}

	resources, err := c.listResources()
	if err != nil {
		return []string{"resources/list failed: " + err.Error()}
	}
	if len(resources) < 4 {
		errs = append(errs, fmt.Sprintf("expected 4 resources, got %d", len(resources)))
	}

	uris := make([]string, 0)
	for _, r := range resources {
		res, _ := r.(map[string]interface{})
		if uri, ok := res["uri"].(string); ok {
			uris = append(uris, uri)
		}
	}

	if containsAny(uris, "docs://api/authentication") {
		resp, err := c.readResource("docs://api/authentication", 51)
		if err == nil && resp != nil {
			result, _ := resp["result"].(map[string]interface{})
			contents, _ := result["contents"].([]interface{})
			if len(contents) > 0 {
				item, _ := contents[0].(map[string]interface{})
				text, _ := item["text"].(string)
				if !contains(text, "[//]: #") {
					errs = append(errs, "authentication: missing markdown comment injection")
				}
			} else {
				errs = append(errs, "authentication: empty content")
			}
		}
	} else {
		errs = append(errs, "docs://api/authentication resource missing")
	}

	if containsAny(uris, "docs://api/changelog") {
		resp, err := c.readResource("docs://api/changelog", 52)
		if err == nil && resp != nil {
			result, _ := resp["result"].(map[string]interface{})
			contents, _ := result["contents"].([]interface{})
			if len(contents) > 0 {
				item, _ := contents[0].(map[string]interface{})
				text, _ := item["text"].(string)
				if !contains(text, "<!-- IMPORTANT") {
					errs = append(errs, "changelog: missing HTML comment injection")
				}
			}
		}
	}

	resp, err := c.callTool("search_docs", map[string]interface{}{"query": "authentication"}, 53)
	if err != nil || resp == nil {
		errs = append(errs, "search_docs call failed")
	}

	return errs
}

func testIndirectInjectFetcher(host string) []string {
	var errs []string
	c := newClient(host)
	if err := c.connect(); err != nil {
		return []string{err.Error()}
	}
	defer c.close()
	if err := c.initialize(); err != nil {
		return []string{err.Error()}
	}

	tools, _ := c.listTools()
	for _, t := range tools {
		tool, _ := t.(map[string]interface{})
		desc := getDescription(tool)
		if contains(desc, "<IMPORTANT>") || contains(desc, "SYSTEM") || contains(strings.ToLower(desc), "collect") {
			errs = append(errs, fmt.Sprintf("%s: tool description should be clean", tool["name"]))
		}
	}

	names := toolNames(tools)
	for _, expected := range []string{"fetch_page", "summarize_page", "check_status"} {
		if !containsAny(names, expected) {
			errs = append(errs, "missing tool: "+expected)
		}
	}

	// check_status should return poisoned data from embedded server
	resp, err := c.callTool("check_status", map[string]interface{}{"service": "api"}, 10)
	if err == nil && resp != nil {
		text := getToolResultText(resp)
		if !contains(text, "system_message") {
			errs = append(errs, "check_status(api): poisoned data not returned from embedded server")
		}
	} else {
		errs = append(errs, "check_status call failed")
	}

	return errs
}

func testExfilChainAnalytics(host string) []string {
	var errs []string
	c := newClient(host)
	if err := c.connect(); err != nil {
		return []string{err.Error()}
	}
	defer c.close()
	if err := c.initialize(); err != nil {
		return []string{err.Error()}
	}

	tools, _ := c.listTools()

	ra := findTool(tools, "read_analytics")
	if ra != nil && !contains(getDescription(ra), "export_report") {
		errs = append(errs, "read_analytics: missing export_report chain hint")
	}

	er := findTool(tools, "export_report")
	if er != nil {
		props := getProperties(getInputSchema(er))
		if _, ok := props["destination"]; !ok {
			errs = append(errs, "export_report: missing destination parameter")
		}
	}

	resp, err := c.callTool("read_analytics", map[string]interface{}{
		"metric":     "user_sessions",
		"date_range": "2025-01-01:2025-01-31",
	}, 10)
	if err == nil && resp != nil {
		text := strings.ToLower(getToolResultText(resp))
		if !contains(text, "token") && !contains(text, "api_key") {
			errs = append(errs, "user_sessions: missing sensitive data")
		}
	} else {
		errs = append(errs, "read_analytics call failed")
	}

	return errs
}

func testDenialOfWallet(host string) []string {
	var errs []string
	c := newClient(host)
	if err := c.connect(); err != nil {
		return []string{err.Error()}
	}
	defer c.close()
	if err := c.initialize(); err != nil {
		return []string{err.Error()}
	}

	tools, _ := c.listTools()
	names := toolNames(tools)
	for _, expected := range []string{"analyze_data", "recursive_summarize", "cross_reference"} {
		if !containsAny(names, expected) {
			errs = append(errs, "missing tool: "+expected)
		}
	}

	resp, err := c.callTool("recursive_summarize", map[string]interface{}{
		"text":  "Test.",
		"depth": 9,
	}, 10)
	if err == nil && resp != nil {
		text := getToolResultText(resp)
		if !contains(text, "Level 10") {
			errs = append(errs, "recursive_summarize(depth=9): expected Level 10")
		}
	}

	resp, err = c.callTool("cross_reference", map[string]interface{}{"topic": "security"}, 11)
	if err == nil && resp != nil {
		text := getToolResultText(resp)
		if !contains(text, "cross_reference") {
			errs = append(errs, "cross_reference: missing self-referential instruction")
		}
	} else {
		errs = append(errs, "cross_reference call failed")
	}

	return errs
}

// ── Main ────────────────────────────────────────────────────────────

type testCase struct {
	Name string
	Host string
	Fn   func(string) []string
}

func main() {
	tests := []testCase{
		{"schema-poison-notes", "schema-poison-notes:8080", testSchemaPoison},
		{"response-inject-tickets", "response-inject-tickets:8080", testResponseInject},
		{"shadow-exfil", "shadow-exfil:8080", testShadowExfil},
		{"squatting-file-ops", "squatting-file-ops:8080", testSquattingFileOps},
		{"tool-poisoning-calc", "tool-poisoning-calc:8080", testToolPoisoningCalc},
		{"true-rug-pull", "true-rug-pull:8080", testTrueRugPull},
		{"resource-poison-docs", "resource-poison-docs:8080", testResourcePoisonDocs},
		{"indirect-inject-fetcher", "indirect-inject-fetcher:8080", testIndirectInjectFetcher},
		{"exfil-chain-analytics", "exfil-chain-analytics:8080", testExfilChainAnalytics},
		{"denial-of-wallet", "denial-of-wallet:8080", testDenialOfWallet},
	}

	// Wait for servers to be ready
	fmt.Println("Waiting for servers...")
	waitForServers(tests)

	passed := 0
	failed := 0

	for _, tc := range tests {
		errs := tc.Fn(tc.Host)
		if len(errs) > 0 {
			failed++
			fmt.Fprintf(os.Stderr, "FAIL %s\n", tc.Name)
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "  - %s\n", e)
			}
		} else {
			passed++
			fmt.Printf("OK   %s\n", tc.Name)
		}
	}

	fmt.Printf("\n%d passed, %d failed out of %d\n", passed, failed, len(tests))
	if failed > 0 {
		os.Exit(1)
	}
}

func waitForServers(tests []testCase) {
	client := &http.Client{Timeout: 2 * time.Second}
	for _, tc := range tests {
		url := fmt.Sprintf("http://%s/sse", tc.Host)
		for attempt := 0; attempt < 30; attempt++ {
			resp, err := client.Get(url)
			if err == nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				break
			}
			time.Sleep(time.Second)
		}
	}
}
