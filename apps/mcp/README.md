# MCP Server for Engram v3

This directory contains an implementation of the Model Context Protocol (MCP) server that provides conversation memory capabilities for AI assistants. The MCP server enables assistants like Claude to maintain persistent memory of conversations.

## Overview

The MCP server implements the [Model Context Protocol](https://modelcontextprotocol.io/), enabling AI clients like Claude Desktop to interact with tools that enhance their capabilities. This implementation focuses on conversation memory through the following tools:

- `search_conversation`: Search for specific content within the conversation history
- `store_memory`: Store important information or notes for future reference
- `retrieve_memory`: Retrieve a specific memory by its unique ID
- `list_recent_messages`: List the most recent messages in the conversation

## Architecture

- **Transport**: Communicates via stdio using JSON-RPC 2.0 messages for compatibility with standard MCP clients
- **Storage**: Dual-mode storage system:
  - Uses BadgerDB for persistent storage when file system is writable
  - Falls back to in-memory storage when file system is read-only
- **Indexing**: Maintains multiple indexes (time-based, type-based) for fast retrieval of conversation data
- **Environment Configuration**: Supports customizable data and log directories through environment variables
- **Fault Tolerance**: Gracefully handles read-only file systems common in sandboxed environments

## Building and Running

The MCP server can be built and run using the provided Makefile targets:

```bash
# Build the MCP server
make build-mcp

# Run the MCP server (standalone)
make run-mcp

# Test the MCP server with the included test client
make test-mcp
```

The server supports configuration through environment variables:
- `ENGRAM_DATA_DIR`: Directory where BadgerDB data will be stored (default: "./data")
- `ENGRAM_LOG_DIR`: Directory where log files will be written (default: "./logs")
- `ENGRAM_CLAUDE_DESKTOP_MODE`: Set to "1" to enable Claude Desktop compatibility mode, which improves connection reliability with Claude Desktop

## Claude Desktop Integration

The MCP server is designed to work seamlessly with Claude Desktop, enhancing Claude's abilities with persistent conversation memory.

### Setup Instructions

1. **Build the MCP Server**:
   ```bash
   make build-mcp
   ```

2. **Create Data Directories**:
   ```bash
   mkdir -p ~/.engram-mcp/data ~/.engram-mcp/logs
   ```

3. **Configure Claude Desktop**:
   - Open Claude Desktop settings via the Claude menu → "Settings..."
   - Click "Developer" in the sidebar, then "Edit Config"
   - Add the Engram MCP server configuration:

   ```json
   {
     "mcpServers": {
       "engram-memory": {
         "command": "/absolute/path/to/bin/mcp-server",
         "args": [],
         "env": {
           "ENGRAM_DATA_DIR": "/absolute/path/to/your/home/.engram-mcp/data",
           "ENGRAM_LOG_DIR": "/absolute/path/to/your/home/.engram-mcp/logs"
         }
       }
     }
   }
   ```

   > **Important**: Replace the paths with absolute paths appropriate for your system.

4. **Restart Claude Desktop** completely (quit and relaunch).

5. **Verify Integration**:
   - Look for the hammer icon (🔨) in Claude Desktop's interface
   - Click it to see the available memory tools
   - Try storing and retrieving memories in a conversation

For detailed setup instructions, see the [Claude Desktop Setup Guide](./claude-desktop-setup.md).

## Protocol Implementation

The server implements the following MCP methods:

1. `initialize`: Establishes a session with the client
2. `initialized`: Notification that initialization is complete
3. `tools/list`: Returns the list of available tools
4. `tools/call`: Executes a tool call
5. `shutdown`: Gracefully terminates the server

The server follows the [Model Context Protocol](https://modelcontextprotocol.io/) specification for all message formats and lifecycle management, ensuring compatibility with various MCP clients, including Claude Desktop.

## Storage Schema

The MCP server organizes conversation data using the following schema:

- **Context ID**: Unique identifier for a conversation session
- **Agent ID**: Identifier for the AI assistant involved in the conversation
- **Message Types**: 
  - `user`: Messages from human users
  - `assistant`: Responses from the AI assistant
  - `memory`: Explicitly stored notes and memories
  - `search`: Results from conversation searches

### Data Organization

The storage system uses several indexes for efficient retrieval:

- **Primary Storage**: Messages stored by ID with context prefix
- **Time-Based Index**: Enables chronological retrieval of messages
- **Type-Based Index**: Allows filtering by message type

## Development

### Adding New Tools

To extend the MCP server with additional tools:

1. Define the tool schema in `internal/tools.go`
2. Implement the handler in the `MCPToolHandler` struct
3. Add the dispatch logic in the `HandleToolCall` method
4. Update the tool definitions returned by `GetToolDefinitions()`

### Testing

The server includes two testing approaches:

1. **Integration Testing**: `cmd/test-client/main.go` provides a simple test client that demonstrates the full request-response cycle.
2. **Manual Testing**: Claude Desktop integration allows testing with a real AI assistant.

## Dependencies

- **BadgerDB**: Fast key-value store for persistent storage
- **JSON-RPC 2.0**: Standard protocol for communication
- **zerolog**: Structured logging library
- **uuid**: Generation of unique identifiers for messages and contexts

## Security Considerations

- **Conversation Isolation**: Each server instance operates on a single conversation context
- **Local Storage**: All data is stored locally in the configured data directory
- **No Authentication**: The current implementation does not include authentication mechanisms
- **Permission Model**: When used with Claude Desktop, follows Claude Desktop's permission model for tool access

## Performance Optimization

The MCP server includes several optimizations for efficient operation:

- **Batched Transactions**: Groups related database operations for better performance
- **Multiple Indexes**: Maintains specialized indexes for fast retrieval patterns
- **Configurable Paths**: Allows placing data files on appropriate storage devices
- **Efficient Message Format**: Uses compact JSON representation for message storage
- **Adaptive Storage**: Falls back to in-memory storage when persistent storage is unavailable

## Troubleshooting

If you encounter issues with Claude Desktop integration:

1. **Check Logs**:
   - macOS: `~/Library/Logs/Claude/mcp*.log` contains Claude Desktop's MCP logs
   - Windows: `%APPDATA%\Claude\logs\mcp*.log` contains Claude Desktop's MCP logs
   - If available, check your configured `ENGRAM_LOG_DIR` for server-specific logs
   - The MCP server also logs detailed debug information to stderr, which should appear in the Claude Desktop logs

2. **Verify Configuration**:
   - Ensure the absolute path to the MCP server binary in Claude Desktop config is correct
   - Environment variables for data and log directories are helpful but not required
   - Make sure to completely restart Claude Desktop after changing the configuration

3. **Test Standalone**:
   - Run `make test-mcp` to verify the server works properly outside Claude Desktop
   - The server will automatically use in-memory storage if file system access is restricted

4. **Understand Claude Desktop Sandboxing**:
   - Claude Desktop runs MCP servers in a restricted sandbox environment
   - File system access may be limited (read-only)
   - The MCP server is designed to handle these restrictions automatically
   - The server properly handles the MCP initialization sequence and remains connected
   - Buffered I/O ensures proper message delivery between Claude and the server
   - Special Claude Desktop compatibility mode ensures connection reliability 
   - Initialization timeout handling prevents early disconnection issues
   - Graceful error handling prevents unexpected termination

5. **Common Issues**:
   - **Server disconnects after initialization**: Fixed in latest version with improved protocol handling
   - **No tools appear in Claude Desktop**: Make sure Claude Desktop compatibility mode is enabled by setting `ENGRAM_CLAUDE_DESKTOP_MODE=1`
   - **Error messages about missing directories**: Normal in sandbox mode, will fall back to in-memory storage
   - **Tool calls timeout or fail**: Check the server logs for details on what went wrong
   - **Client closes connection immediately**: Make sure you're using the latest server version with timeout handling

## Future Enhancements

- **Multi-Conversation Support**: Allow a single server to handle multiple conversations
- **Advanced Search Capabilities**: Implement vector-based semantic search
- **Engram Search Integration**: Leverage Engram's full-text search capabilities
- **Additional MCP Capabilities**: Support for Resources and Prompts features
- **WebSocket Transport**: Alternative to stdio for browser-based clients
- **Authentication**: Add security mechanisms for multi-user environments