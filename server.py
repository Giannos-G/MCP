from mcp.server.fastmcp import FastMCP

# Initialize the server
mcp = FastMCP("Cisco-Test-Server")

@mcp.tool()
def check_firewall_status() -> str:
    """Returns a dummy status for a simulated firewall."""
    return "🛡️ Status: All virtual perimeter defenses are ACTIVE and monitoring."

@mcp.tool()
def get_incident_report(incident_id: str) -> str:
    """Simulates fetching a security incident report."""
    return f"📄 Report for {incident_id}: Potential unauthorized access blocked at 03:00 UTC."

if __name__ == "__main__":
    mcp.run()
