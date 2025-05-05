- Never do mock implementations
- Always use Badger storage implementation, not SQLite or RocksDB
- Use JSON marshaling instead of protobuf marshaling (proto.Marshal)
- Use ./test.sh for running tests with emoji indicators
- Core tests (storage, search, lockmanager, router, API) are now working with Badger
- Always update todo.md file after the work is done

## Component Architecture
- Engine → Main coordinator
- Storage → Badger-based persistent storage (no SQLite, no RocksDB)
- Router → Event routing system
- Notifier → Real-time notifications via WebSocket/SSE
- Lock Manager → Resource locking mechanism
- Search → Text and metadata indexing (using Badger)
- API → HTTP/WebSocket endpoints

## Component Wiring
1. Create and start storage
2. Create router and connect it to storage event stream
3. Create search and connect it to router subscription
4. Create lock manager with storage
5. Create notifier with router and start it with router subscription
6. Create API with all components
7. Start API server