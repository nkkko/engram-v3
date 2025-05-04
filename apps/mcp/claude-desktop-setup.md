# Setting Up Engram Memory MCP Server with Claude Desktop

This guide will help you integrate your Engram Memory MCP server with Claude Desktop for enhanced conversation memory capabilities.

## Prerequisites

- [Claude Desktop](https://claude.ai/download) installed on your computer
- The Engram v3 project cloned and built

## Step 1: Build the Engram MCP Server

First, build the Engram MCP server binary:

```bash
# From the engram-v3 project root
make build-mcp
```

This will create the `mcp-server` binary in the `./bin` directory. Make sure the build completes successfully before proceeding.

## Step 2: Create Data Directories (Optional)

The MCP server can now run in a fully in-memory mode when file system access is restricted. However, if you want persistent storage, create directories for the MCP server data:

```bash
mkdir -p ~/.engram-mcp/data
mkdir -p ~/.engram-mcp/logs
```

> **Note**: Even if these directories cannot be created or accessed by Claude Desktop, the MCP server will still function using in-memory storage. Your conversation memory will be available during the current session but will not persist after Claude Desktop is closed.

## Step 3: Configure Claude Desktop

1. Open Claude Desktop
2. Click on the Claude menu in your system menu bar (macOS) or system tray (Windows)
3. Select "Settings..."
4. Click on "Developer" in the left sidebar
5. Click "Edit Config"

This will open the Claude Desktop configuration file. Replace its contents with the following:

```json
{
  "mcpServers": {
    "engram-memory": {
      "command": "/path/to/your/engram-v3/bin/mcp-server",
      "args": [],
      "env": {
        "ENGRAM_DATA_DIR": "/Users/yourusername/.engram-mcp/data",
        "ENGRAM_LOG_DIR": "/Users/yourusername/.engram-mcp/logs",
        "ENGRAM_CLAUDE_DESKTOP_MODE": "1"
      }
    }
  }
}
```

Make sure to replace:
- `/path/to/your/engram-v3/bin/mcp-server` with the absolute path to your compiled MCP server binary
- `/Users/yourusername/.engram-mcp/data` with the absolute path to your data directory
- `/Users/yourusername/.engram-mcp/logs` with the absolute path to your logs directory

On Windows, use appropriate paths like `C:\Users\yourusername\.engram-mcp\data`.

## Step 3: Restart Claude Desktop

After saving the configuration file, completely restart Claude Desktop:

1. Quit Claude Desktop from the Claude menu
2. Start Claude Desktop again

## Step 4: Verify the Integration

After restarting Claude Desktop:

1. Start a new conversation
2. Look for the hammer icon (🛠️) in the input box area
3. Click on the hammer icon to see the available tools
4. You should see the Engram memory tools:
   - search_conversation
   - store_memory
   - retrieve_memory
   - list_recent_messages

## Step 5: Use the Memory Tools

Here are some examples of how to use the memory tools:

1. **Store Important Information**:
   - Ask Claude: "Please store this important information: The project deadline is May 15, 2025"
   - Claude should use `store_memory` and confirm it saved the information

2. **Retrieve Recent Messages**:
   - Ask Claude: "What were the last few messages in our conversation?"
   - Claude should use `list_recent_messages` to show the conversation history

3. **Search for Specific Content**:
   - Ask Claude: "Can you find any mentions of 'deadline' in our conversation?"
   - Claude should use `search_conversation` to find relevant messages

4. **Retrieve a Specific Memory**:
   - If you know a memory ID, ask Claude: "Can you retrieve the memory with ID [ID]?"
   - Claude should use `retrieve_memory` to fetch that specific memory

## Troubleshooting

If you encounter issues:

1. **Server Not Appearing in Claude Desktop**:
   - Check the configuration file path is correct
   - Verify the binary was built successfully
   - Ensure you restarted Claude Desktop completely
   - Make sure the server binary has execute permissions (`chmod +x bin/mcp-server`)

2. **Check Claude Desktop Logs**:
   - macOS: Check logs at `~/Library/Logs/Claude/mcp*.log`
   - Windows: Check logs at `%APPDATA%\Claude\logs\mcp*.log`
   - Examine the logs for details about what went wrong

3. **Test the MCP Server Directly**:
   - Run the test client to verify the server works:
     ```bash
     make test-mcp
     ```
   - This will simulate the Claude Desktop interaction with the server

4. **Common Errors**:
   - "Command not found": Check the binary path in the Claude Desktop configuration
   - "Read-only file system": This is expected and handled automatically with in-memory storage
   - "Failed to initialize storage": Not critical - the server will fall back to in-memory mode
   - "Connection closed": The server may have exited unexpectedly (check error messages)

5. **Protocol Support**:
   - The latest version of the server handles the complete MCP protocol lifecycle correctly
   - The server now maintains a persistent connection with Claude Desktop
   - It supports the full initialization sequence and properly implements all required methods
   - Uses a robust channel-based approach for message handling
   - Properly flushes all responses to ensure they're immediately received by Claude
   - Implements graceful shutdown when requested by the client

6. **Performance Notes**:
   - In-memory storage is fast but doesn't persist across Claude Desktop restarts
   - For best results, try to provide writable directories for the data and logs in your configuration

## How It Works

When you interact with Claude Desktop:

1. Claude analyzes your message to determine if memory tools could help
2. If needed, Claude calls the Engram MCP server via the stdio transport
3. The MCP server processes the request using either:
   - BadgerDB persistent storage (if file system is writable)
   - In-memory storage (if file system is read-only)
4. Results are returned to Claude, which uses them to enhance its responses

Your conversations and memories are stored according to the storage mode:
- With persistent storage, they remain available across Claude Desktop restarts
- With in-memory storage, they persist only during the current Claude Desktop session