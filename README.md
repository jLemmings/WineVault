# WineVault

A responsive Nuxt 3 wine cellar planner with an interactive floor plan, full rack views, searchable collection, bottle details, and persistent inventory managed by Go.

## Run locally

## Project layout

```text
frontend/             Nuxt app, assets, npm dependencies, browser checks
backend/              Go API, PostgreSQL migrations, integration tests
docker-compose.yml    Development PostgreSQL service and persistent volume
```

Requires Node.js 20.19+ (or 22.12+), Go 1.23+, and Docker with Compose.

Start PostgreSQL from the repository root:

```sh
docker compose up -d --wait
```

Start the backend in one terminal:

```sh
cd backend
go run .
```

Start Nuxt in another terminal:

```sh
cd frontend
npm ci
npm run dev
```

Open http://localhost:3000. Nuxt proxies `/api` to the Go server on port 8080.

## Owner account and sign-in

WineVault has exactly one owner account, stored in PostgreSQL. Restart the backend after updating to apply migration 006. Until the account is created, startup prints a one-time setup code in the backend terminal. Open the app, enter that code, choose a username and a password, and confirm the password. Setup signs you in and closes account registration. If you restart before completing setup, use the newly printed code. Existing cellar data stays in place.

Passwords require at least 12 characters in the form and at most 72 UTF-8 bytes. The backend stores a bcrypt hash, never the password. Sessions last seven days and use HttpOnly, SameSite=Strict cookies; session tokens are hashed in PostgreSQL. All cellar APIs, including scans, history, and health checks, require a valid session. Authentication attempts are limited to ten per minute for this single-owner installation.

Open **Preferences** beside your sidebar profile to **Change password**. Enter your current password and confirm the new one. Changing it signs out all devices; sign back in with the new password. Use **Sign out** in the top bar or Preferences to end your current session.

For HTTPS deployments, set `AUTH_COOKIE_SECURE=true` in `backend/.env`, restart the backend, and serve the frontend and `/api` through the same HTTPS origin. Local HTTP development uses `false`. Authentication writes and other API mutations require the `X-WineVault-Request: 1` header; the frontend adds it automatically. No cross-origin API access is enabled. Keep the initial setup code private. There is no public registration or email password-reset flow.

Authentication validation: run `go test ./...` from `backend/` with `TEST_DATABASE_URL` set for database integration tests. Run `node tests/auth.mjs` from `frontend/` against the running frontend for the mocked browser flow; it creates no real owner account.

The live browser, editor, and scanner suites require `WINEVAULT_TEST_USERNAME` and `WINEVAULT_TEST_PASSWORD` for an already configured test installation. They sign in using those credentials and never automatically create an owner account. As before, these live suites modify inventory and then clean up their test bottles; those additions/removals also appear in history. Use a test database for them.

## GrapeMinds wine information

Set `GRAPEMINDS_API_KEY=your-key` in `backend/.env` and restart the backend. Migration 007 creates durable wine information records and links existing inventory; migration 008 caches search candidates. This integration uses the account's existing permission to store catalogue data, as confirmed during setup. It does not call the separately billed licence-purchase endpoint.

After a wine is added manually or through scanning, WineVault searches GrapeMinds and fetches its details. An exact, unique name match is attached automatically; ambiguous matches can be chosen in the bottle's **Wine information** section. Wine details are supplementary and do not replace your reviewed name, vintage, or location. Stored information is shared across vintages by bottles with the same wine name, region, and type (ignoring capitalization and repeated spaces). Migration 009 merges existing cache records, preserving the most recent saved details, so all matching bottles show the loaded status without another API request. Full detail JSON, provider ID, timestamps, status, and failures are stored in PostgreSQL. API failures do not undo a bottle addition, and failed reloads preserve previously fetched information.

Existing wines show **Fetch information**. Missing or failed results show **Retry fetch**; **Reload from GrapeMinds** refreshes existing details. Search for the producer and wine name to choose or correct a catalogue match. Temperature advice is excluded from the displayed provider text.

Request usage: a new match normally costs one search plus one details request, including when you choose from cached ambiguous results. Multiple bottles in one batch share that lookup. Opening saved details or adding more identical bottles uses zero additional provider requests. An explicit reload of an already matched wine uses one details request. A different search or retry can use additional requests. The backend logs each provider call's method, endpoint (without search text), and HTTP status; it never logs the API key or raw error response.

Calls use a bounded timeout and are spaced apart; HTTP 429 responses defer subsequent requests. Data is requested in English. The documented detail endpoint supplies catalogue-level information, so it is not presented as vintage-specific analysis. No extra drinking-window, region, or producer-insight endpoints are fetched. See the [GrapeMinds API reference](https://www.grapeminds.eu/developers/endpoints).

Run `node tests/information.mjs` from `frontend/` for the mocked browser checks. The Go integration tests verify persistence, shared caching, retries, and exact request counts with a mock provider; they make no live GrapeMinds calls.

## Photo and barcode scanner

Choose **Scan label** (or **Fill from a label photo** in Add wine). The scanner offers two modes:

- **Label photo:** take a smartphone photo, upload an image, or drop one into the preview. Choose **Identify wine**, review and correct the suggested wine name, vintage, region, and type, then choose a shelf/slot and **Add to cellar**. Unknown years remain blank; explicitly non-vintage bottles can be saved as NV.
- **Barcode:** scan a live camera view, take/upload a barcode photo, or type its number. EAN-13, EAN-8, and UPC-A are decoded locally, including a JavaScript fallback when the browser lacks a native barcode detector. Images and live camera frames are not sent for barcode lookup. Only the validated number is sent to the Go API.

Barcode lookup first checks previously saved bottles in PostgreSQL, then queries the public Open Food Facts catalogue. No OpenAI key is needed for barcode scanning/lookup. Coverage is incomplete: unknown codes and products not classified as wine offer a label-photo/manual fallback. A barcode can be shared across vintages, so the year is always left for the user to confirm. Saving a barcode-identified bottle keeps its barcode for future lookups. UPC-A is stored in its equivalent leading-zero EAN-13 form.

The scanner never inserts inventory automatically. In the review, set **Number of bottles**, choose a shelf, and select one free slot for each bottle in the shelf grid. **Select first available** fills the selection for you. All bottles share the reviewed wine details and are saved together when **Add to cellar** succeeds. If a slot becomes occupied, no bottles in the batch are added; review the refreshed selection and try again. Barcode catalogue attribution is shown in the review. Open Food Facts data is available under ODbL; see the [API documentation](https://openfoodfacts.github.io/openfoodfacts-server/api/) for usage and attribution details. The backend spaces public catalogue requests at least four seconds apart.

### Enable label recognition

Label-photo recognition uses the OpenAI Responses API with image input and structured output. Create a backend-only settings file once:

```powershell
cd backend
Copy-Item .env.example .env
```

Edit `backend/.env` and set `OPENAI_API_KEY=your-api-key`, then run `go run .` from `backend/`. The backend automatically reads `.env` from its working directory on every startup; restart it after changing the file. Existing process environment variables take precedence. A missing file is allowed; an unreadable or malformed file stops startup. `.env` is ignored by Git. The key must not be committed or exposed in Nuxt public configuration. Without a key, barcode lookup and manual entry still work, and label recognition shows a setup message. Requests may incur provider usage charges. Implementation references: [image inputs](https://developers.openai.com/api/docs/guides/images-vision) and [structured outputs](https://developers.openai.com/api/docs/guides/structured-outputs).

Photos are resized to a maximum 1800-pixel edge in the browser. The original selection may be up to 20 MB; the Go endpoint accepts a single JPEG/PNG of at most 8 MB and 16 megapixels. The server verifies and re-encodes images, stripping EXIF metadata before recognition. WineVault does not persist uploaded photos. OpenAI requests use `store: false`. HEIC support depends on the browser; unsupported images must be exported as JPEG.

### Use a smartphone

Run Nuxt with `npm run dev` in `frontend/` and open `http://<your-computer-LAN-IP>:3000` from a phone on the same Wi-Fi (allow the development server through your firewall). Native **Take a photo** and **Take barcode photo** inputs request the rear camera on supported mobile browsers. For **live barcode video**, browsers require HTTPS or localhost: serve the app through your HTTPS development proxy, or use a barcode photo on a plain LAN HTTP connection. Camera tracks stop on close, tab switch, match, or after 30 seconds without a match.

## Room and shelf editors

Choose **Edit room** in the cellar toolbar to open the bird's-eye room editor:

- Change cellar/room names, width and depth, rectangular or L-shaped outline, and floor material.
- Drag shelves or the tasting table, or select an object and set exact coordinates and dimensions. Arrow keys move selected objects by 10 cm; Shift + arrow moves 50 cm. Grid snapping can be disabled for finer pointer movement.
- Rotate shelf footprints by 90 degrees. Configure the door wall, offset, and width, or hide the tasting table.
- Save applies the whole layout atomically. Reset restores the opening draft; Cancel or Escape discards it.

The plan uses real room dimensions and saved shelf footprints. Validation prevents objects outside the walls, overlapping footprints, and objects blocking the 0.5 m entrance clearance. An L-shaped room has its cutout at the northeast corner.

Choose **Edit shelf** under the selected shelf, or the pencil icon on a rack card, to configure its name, short label, location, wine description, rows, columns, and color. A live preview shows stored bottles and available slots. Shelf capacity is rows × columns (up to 20 each). Existing bottles keep their numeric slot IDs; changing column count changes the displayed row/column addresses. Shrinking a shelf is rejected if any removed slot contains a bottle.

**Add shelf** creates an empty shelf in an available part of the room. Arrange its physical footprint in the room editor. Only empty shelves can be removed, using the shelf editor's Remove action.

All editor changes are stored in PostgreSQL. Revision checks prevent an older editor from overwriting newer room or shelf changes. The version 2 migration adds editor settings without changing existing bottle records.

## Database storage

PostgreSQL is the source of truth for cellar name, owner, room dimensions, rack names, capacities, and all bottles. The frontend loads `/api/cellar` as one consistent database snapshot and calculates totals from those records. There is no JSON or hardcoded inventory fallback when the database is unavailable.

The backend automatically applies its embedded SQL migration on startup. It imports `backend/data/bottles.json` if present (when started from `backend/`), otherwise it seeds 82 example bottles across four racks. The import leaves the original file untouched; bottle IDs are reassigned by PostgreSQL. An existing empty JSON inventory stays empty. Invalid imports fail and roll back the migration instead of silently discarding bottles.

Migration records prevent repeated imports/seeding on later starts, including after every bottle has been consumed. Writes commit directly to PostgreSQL. Database constraints prevent duplicate slots and references to nonexistent rack slots.

Database data persists in the Compose-managed `winevault_postgres_data` volume. `docker compose down` stops the database without removing its data. Do not add `--volumes` unless intentionally resetting it.

### Configuration

Defaults work without creating any configuration files:

| Component | Setting | Default |
| --- | --- | --- |
| Go API | `DATABASE_URL` | `postgres://winevault:winevault_dev@127.0.0.1:5432/winevault?sslmode=disable` |
| Go API | `HTTP_ADDR` | `:8080` |
| First migration only | `WINEVAULT_LEGACY_JSON` | `data/bottles.json`, relative to backend working directory |
| Nuxt proxy | API address | `http://127.0.0.1:8080` in `frontend/nuxt.config.ts` |

To change Compose credentials or the published port, copy `.env.example` to `.env` and edit it. Set the matching `DATABASE_URL` in the backend process environment; the Go process does not automatically load Compose's `.env` file. In PowerShell:

```powershell
$env:DATABASE_URL = 'postgres://winevault:winevault_dev@127.0.0.1:5432/winevault?sslmode=disable'
```

The database port is bound only to localhost. The Compose credentials are for local development.

### API

- `GET /api/cellar`: cellar metadata, racks, and bottles from PostgreSQL
- `GET /api/wine-scan/status`: whether label recognition is configured (never exposes the key)
- `POST /api/wine-scan`: multipart image field named `image`; returns suggestions without saving inventory
- `GET /api/barcodes/{code}`: checksum-validated EAN/UPC lookup, saved cellar first and public catalogue second
- `PUT /api/cellar/layout`: save room geometry, appearance, door/table settings, and all shelf footprints (requires current `revision`)
- `PUT /api/racks/{id}`: update shelf metadata and row/column configuration (requires `revision`)
- `POST /api/racks`: create an empty shelf (requires `revision`)
- `DELETE /api/racks/{id}`: remove an empty shelf (JSON body with `revision`)
- `GET /api/bottles`: list bottles
- `POST /api/bottles`: add a bottle with name, vintage (`0` for NV), region, type, rack, zero-based slot, and optional barcode
- `DELETE /api/bottles?id=...`: mark a bottle as enjoyed
- `GET /api/health`: database connectivity (503 when unavailable)

## Validate and build

```sh
cd backend
go test ./...
go vet ./...
cd ../frontend
npm run build
```

PostgreSQL integration tests create and clean up uniquely named schemas without modifying application inventory. Enable them explicitly (PowerShell, from `backend/`):

```powershell
$env:TEST_DATABASE_URL = 'postgres://winevault:winevault_dev@127.0.0.1:5432/winevault?sslmode=disable'
go test -v ./...
```

These verify database reads/writes, metadata loading, concurrent duplicate-slot protection, idempotent migrations, legacy import, and migration rollback. Without `TEST_DATABASE_URL`, database tests are skipped.

With the API and frontend running, execute `node tests/browser.mjs` from `frontend/` for desktop/mobile and inventory flow checks. It uses installed Microsoft Edge and temporarily adds a test bottle before removing it.

Run `node tests/editors.mjs` from `frontend/` to check dragging, room save/reload, draft cancellation, shelf resizing, bottle preservation, creation/removal, and mobile editors. This check temporarily edits the development cellar and restores its original settings afterward. PostgreSQL integration tests additionally cover invalid geometry, occupied-slot protection, stale edits, and migration persistence in isolated test schemas.

`node --test tests/barcode.test.mjs` checks EAN/UPC decoding, rotated images, and invalid checksums without a browser. `node tests/scanner.mjs` checks the photo-review-save flow with mocked recognition and real database writes, cleaning up its test bottle. Go scanner tests mock the recognition provider and barcode catalogue; live recognition accuracy and physical camera behavior must be checked with a configured API key and a real phone.

For production, run the Go service alongside `node .output/server/index.mjs` from `frontend/`. This is a single-user local application; add authentication and access controls before public deployment.

## Wine history

To relocate a bottle, open it from Wine Collection or a rack, choose **Move bottle**, select the destination rack and an empty slot, and choose **Confirm move**. If several bottles match, first select the exact bottle in the highlighted front view. Moves preserve the bottle's identity and do not count as additions or enjoyed bottles. Restart the backend after updating to enable the move endpoint.

Choose **History** in the sidebar to see dated additions and enjoyed bottles, filter by activity, and load older entries. Each bottle keeps a snapshot of its wine details and shelf address even after it is enjoyed or its shelf changes. Restart the backend to apply migration 005 and enable tracking. Earlier additions and removals were not recorded and cannot be reconstructed; existing inventory is not assigned invented addition dates. Batch history commits with the bottles, so failed additions do not create history entries.

