# Simamia development notes

## Browser and desktop behavior

The browser UI uses the Go API at `http://localhost:8080/api/v1` by default; override it with `NEXT_PUBLIC_SIMAMIA_API_URL`. Browser sign-in and account creation use local Simamia accounts stored by that API. Business data is SQLite-backed and scoped to the signed-in account. If the API is unavailable, the browser can still show its local demo preview, but account registration and saved server records require the API.

The Wails desktop build uses the same screens with a device-local workspace and per-user SQLite database. Desktop account/session auth is intentionally bypassed inside the local Wails app; no internet is needed for local record operations, and desktop data does not sync to browser accounts. SMS, email, mobile-money processing and AI-provider calls remain unimplemented.

The API stores PBKDF2 password hashes and hashed bearer-session tokens in SQLite. Sessions last 12 hours by default or 30 days when “Remember me” is selected. Password reset and account recovery are not implemented. Keep local development data synthetic.

## Local startup

Start the API in one terminal:

```bash
cd api
go test ./...
go run ./cmd/simamia-api
```

Start the UI in another terminal:

```bash
NEXT_PUBLIC_SIMAMIA_API_URL=http://localhost:8080/api/v1 npm run dev -- --hostname 0.0.0.0
```

Open `http://localhost:3000`, choose **Create account**, and enter a name, Tanzania phone number, and password. The dashboard's sample data remains local to the browser; records created through the API are stored per account. Use **API & docs** to see whether the configured API is reachable.

## Authenticated API workflow

Set the API base URL, create a throwaway local account, and capture its returned bearer token (requires `jq`):

```bash
export API_BASE=http://localhost:8080/api/v1
export TOKEN=$(curl -sS -X POST "$API_BASE/auth/register" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Demo Owner","phone":"+255 700 000 010","password":"demo-password-2026","remember_me":false}' \
  | jq -r '.data.access_token')
```

Confirm the account session and test health:

```bash
curl -sS "$API_BASE/health"
curl -sS "$API_BASE/auth/me" -H "Authorization: Bearer $TOKEN"
```

Create a fabricated customer, then create an order:

```bash
curl -sS -X POST "$API_BASE/customers" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Demo Customer Created","phone":"+255 700 000 011","email":"demo.created@example.test","location":"Dar es Salaam (demo)"}'

curl -sS -X POST "$API_BASE/orders" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"customer":"Demo Customer Created","service":"Laundry · Express","date":"2026-10-12T10:00:00+03:00","amount":48000,"status":"New","method":"M-Pesa","assignee":"Demo Staff 01"}'
```

For login, send the same phone and password to `POST /auth/login`. When finished, revoke the session and clear the shell variable:

```bash
curl -sS -X POST "$API_BASE/auth/logout" -H "Authorization: Bearer $TOKEN"
unset TOKEN
```

Update or delete the returned order using its `data.id`; browse the [OpenAPI contract](../api/openapi.yaml) for all fields and response envelopes. Only use destructive requests with demo records. The API store is shared and persistent, so deletes affect the database until the record is recreated or the local database is intentionally reset.

## Supported endpoints

- `POST /auth/register`, `POST /auth/login`, `GET /auth/me`, `POST /auth/logout`
- `GET /health`
- `GET, POST /businesses`, `GET, PATCH, DELETE /businesses/{id}`
- `GET, POST /orders`, `GET, PATCH, DELETE /orders/{id}`
- `GET, POST /customers`, `GET, PATCH, DELETE /customers/{id}`
- `GET, POST /services`, `GET, PATCH, DELETE /services/{id}`
- `GET, POST /team`, `GET, PATCH, DELETE /team/{id}`
- `GET, POST /expenses`, `GET, PATCH, DELETE /expenses/{id}`
- `GET /payments` (derived and read-only)

All business-data endpoints require a local account bearer token. Collections accept optional `q`, `status`, `page`, and `page_size` query parameters where applicable. All JSON responses include `data` and `meta`; errors include an `error` object and `meta`.
