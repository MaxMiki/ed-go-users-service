# go-users-service

A lightweight Go HTTP service that stores users in MongoDB and exposes a REST API for creating, listing, retrieving, updating, and deleting user records.

## Features

- CRUD operations for `User` resources
- Graceful shutdown on interrupt/terminate signals
- Health check endpoint
- MongoDB-backed persistence
- Bruno API collection for manual testing

## Requirements

- Go 1.22 or later
- MongoDB (local or remote)
- Docker and Docker Compose (optional, for local MongoDB)

## Quick start

1. Start a local MongoDB instance:

   ```bash
   ./start-local-env.sh
   ```

   Or run Docker Compose directly:

   ```bash
   docker compose up -d
   ```

2. Run the service:

   ```bash
   go run main.go
   ```

   The server listens on `http://localhost:8080` and connects to `mongodb://localhost:27017` by default.

## Configuration

| Variable     | Description                              | Default                       |
| ------------ | ---------------------------------------- | ----------------------------- |
| `MONGODB_URI` | MongoDB connection string                | `mongodb://localhost:27017`    |

## API

### Endpoints

| Method | Path            | Description          |
| ------ | --------------- | -------------------- |
| POST   | `/users`        | Create a new user    |
| GET    | `/users`        | List all users       |
| GET    | `/users/{id}`   | Get a user by ID     |
| PUT    | `/users/{id}`   | Update a user by ID  |
| DELETE | `/users/{id}`   | Delete a user by ID  |
| GET    | `/health`       | Health check         |

### Examples

Create a user:

```bash
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com"}'
```

Response:

```json
{
  "id": "64b5f5f...",
  "name": "Alice",
  "email": "alice@example.com",
  "createdAt": "2026-07-21T00:00:00Z",
  "updatedAt": "2026-07-21T00:00:00Z"
}
```

List users:

```bash
curl http://localhost:8080/users
```

Get a user:

```bash
curl http://localhost:8080/users/{id}
```

Update a user:

```bash
curl -X PUT http://localhost:8080/users/{id} \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice Smith","email":"alice.smith@example.com"}'
```

Delete a user:

```bash
curl -X DELETE http://localhost:8080/users/{id}
```

Health check:

```bash
curl http://localhost:8080/health
```

## Testing with Bruno

The `bruno/` directory contains a Bruno collection with ready-to-use requests for all endpoints. Open the collection in the Bruno app or CLI to interact with the service.

## Project structure

```
.
├── bruno/              # Bruno API collection
├── docker-compose.yml  # Local MongoDB setup
├── go.mod
├── go.sum
├── main.go             # Application entry point and HTTP handlers
├── README.md
└── start-local-env.sh  # Helper script to start MongoDB
```

## License

MIT
