# MCP Server Implementation (`apps/mcp/TODO.md`)

Simplified task list for implementing an MCP (Model Context Protocol) server using `stdio` transport within the `engram-v3` project. This server uses Engram's BadgerDB storage for conversation memory and offers "remember" tools.

## Phase 1: Setup & Core MCP (`apps/mcp`) ✅

-   [x] **Project Setup:** Create `apps/mcp` directory, `main.go`, and necessary sub-packages. Add MCP SDK dependencies to main `go.mod`.
-   [x] **Define Storage Schema:** Decide how conversations (user/assistant turns) are mapped to `proto.WorkUnit`s in BadgerDB (using `ContextId`, `AgentId`, `Meta`, etc.).
-   [x] **Implement `stdio` Transport:** Set up the main loop in `apps/mcp/main.go` to read JSON-RPC messages from `stdin` and write responses to `stdout`.
-   [x] **Implement MCP Lifecycle:** Handle `initialize` (respond with server info and capabilities like `tools`) and `initialized` messages.
-   [x] **Basic Message Handling:** Implement JSON-RPC parsing and routing for incoming requests/notifications. Add basic error responses.
-   [x] **Integrate Logging:** Use `zerolog` for logging within the MCP server.

## Phase 2: Engram Integration & Conversation Memory ✅

-   [x] **Integrate Storage:** Add dependency on `internal/storage` within `apps/mcp`. Configure access to the BadgerDB instance (e.g., via path).
-   [x] **Store Conversation Turns:** Implement logic to take incoming messages (from an MCP client interaction, likely via a tool call later) and save them as `WorkUnit`s using JSON marshaling.
-   [x] **Retrieve Conversation History:** Implement function(s) to query `WorkUnit`s for a given conversation (e.g., by `ContextId`) chronologically.

## Phase 3: Implement "Remember" Tools ✅

-   [x] **Define Tools:** Finalize the input/output schemas for MCP Tools: `search_conversation`, `store_memory`, `retrieve_memory`, `list_recent_messages`.
-   [x] **Implement `tools/list`:** Return the defined tools and their schemas.
-   [x] **Implement `tools/call`:**
    -   [x] Add dispatch logic based on the tool name.
    -   [x] **`search_conversation`:** Implement using `internal/search` or direct BadgerDB queries.
    -   [x] **`store_memory`:** Implement by creating a specific `WorkUnit` type/meta and saving via storage.
    -   [x] **`retrieve_memory`:** Implement by querying storage for specific `WorkUnit`s based on criteria.
    -   [x] **`list_recent_messages`:** Implement using storage retrieval functions.
    -   [x] Add error handling within tool results (`isError: true`).

## Phase 4: Testing & Build ✅

-   [x] **Unit Tests:** Added unit tests for tools and storage implementations.
-   [x] **MCP Client Stub (for testing):** Created test utilities for standard and Claude Desktop-specific testing.
-   [x] **Integration Tests:** Test the full flow: initialize, call tools, verify outputs and DB state.
-   [x] **Makefile:** Added `make build-mcp`, `make run-mcp`, and `make test-mcp-claude` targets.
-   [ ] **Test Script:** Integrate MCP server tests into `./test.sh` or a new script.

## Phase 5: Refinement ⏳

-   [ ] **Configuration:** Implement proper configuration loading (DB path, etc.).
-   [ ] **Metrics:** Integrate `internal/metrics` for tracking MCP server activity.
-   [x] **Documentation:** Created comprehensive `README.md` and `claude-desktop-setup.md`.
-   [x] **Protocol Compliance:** Implemented full MCP protocol support with proper lifecycle handling.
-   [x] **Claude Desktop Integration:** Fixed connection issues and ensured compatibility.

## Next Steps

- Enhance test coverage with additional test cases
- Integrate MCP server tests into the main test script
- Implement configuration loading system
- Add metrics for tracking MCP server activity
- Enhance search capabilities with the Engram search component
- Add support for multiple concurrent conversations
- Create Dockerfile for containerized deployment


