// Command smart-speaker-mcp is an MCP server that lets Claude (or any MCP
// client) control Chromecast / Google Home / Nest speakers over the local
// Wi-Fi network. See README.md for installation and usage.
package main

import (
	"github.com/WeCodeBase/smart-speaker-mcp/internal/mcpserver"
)

func main() {
	mcpserver.Run()
}
