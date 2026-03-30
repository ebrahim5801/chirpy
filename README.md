# Chirpy

A Twitter-like microblogging REST API built with Go and PostgreSQL.

## Requirements

- Go 1.23+
- PostgreSQL
- [goose](https://github.com/pressly/goose) for migrations
- [sqlc](https://sqlc.dev/) if regenerating DB code

## Setup

1. Copy the environment file and fill in the values:

```
DB_URL=postgres://user:password@localhost:5432/chirpy?sslmode=disable
JWT_TOKEN=your-secret-key
POLKA_KEY=your-polka-api-key
PLATFORM=dev
```

2. Run database migrations:

```bash
goose -dir sql/schema postgres "$DB_URL" up
```

3. Build and run:

```bash
go build -o chirpy && ./chirpy
```

The server starts on port `8080`.

---

## API Reference

### Health

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/healthz` | No | Returns `OK` if the server is running |

### Users

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/users` | No | Create a new user |
| PUT | `/api/users` | Bearer token | Update email and password |

**POST /api/users** — request body:
```json
{ "email": "user@example.com", "password": "secret" }
```

**PUT /api/users** — request body (same as above), requires `Authorization: Bearer <token>`.

### Authentication

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/login` | No | Login and receive JWT + refresh token |
| POST | `/api/refresh` | Refresh token | Issue a new access token |
| POST | `/api/revoke` | Refresh token | Revoke the refresh token |

**POST /api/login** — request body:
```json
{ "email": "user@example.com", "password": "secret" }
```

Response includes a short-lived JWT (`token`) and a long-lived `refresh_token`.

**POST /api/refresh / POST /api/revoke** — pass the refresh token as `Authorization: Bearer <refresh_token>`.

### Chirps

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/chirps` | Bearer token | Create a chirp (max 140 chars) |
| GET | `/api/chirps` | No | List all chirps |
| GET | `/api/chirps/{chirpID}` | No | Get a single chirp |
| DELETE | `/api/chirps/{chirpID}` | Bearer token | Delete your own chirp |

`GET /api/chirps` accepts an optional query param `?author_id=<uuid>` to filter by user.

### Webhooks

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/polka/webhooks` | ApiKey | Polka payment event handler |

Requires `Authorization: ApiKey <key>`. Handles `user.upgraded` events to grant Chirpy Red membership.

### Admin

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/admin/metrics` | Shows file server hit count |
| POST | `/admin/reset` | Deletes all users (only works when `PLATFORM=dev`) |
