# Animal Poke Docker deployment

## Architecture

- `backend`: pulled from CNB and bound to `127.0.0.1:8080` by default for a host reverse proxy.
- `mysql`: reachable only through the internal Docker network. It has no host `ports` mapping.
- `mysql_data`: named volume containing all database data.
- `mysql_ca`: named volume containing only the CA certificate mounted by the backend.
- `mysql_tls`: named volume containing the CA private key and MySQL server certificate/key; never mounted by the backend.
- Frontend: build once and upload the contents of `frontend/dist/` to a static host.

The backend has both an egress network for AI/weather APIs and a private internal network for MySQL. MySQL only joins the private network.

## 1. Configure production secrets

```bash
cp deploy/.env.production.example deploy/.env.production
chmod 600 deploy/.env.production
```

Generate every cryptographic secret independently:

```bash
openssl rand -base64 48
```

Set `CORS_ALLOWED_ORIGINS` to the exact frontend origin. Never commit `deploy/.env.production`.

## 2. Start backend and private MySQL

```bash
docker compose \
  --env-file deploy/.env.production \
  -f deploy/compose.production.yml \
  pull

docker compose \
  --env-file deploy/.env.production \
  -f deploy/compose.production.yml \
  up -d
```

Verify:

```bash
curl --fail http://127.0.0.1:8080/readyz
docker compose --env-file deploy/.env.production -f deploy/compose.production.yml ps
```

Do not add a `ports` section to the MySQL service. To inspect MySQL, use `docker compose exec mysql mysql ...` from the host.

## 3. Persistence and backup

`animal-poke-mysql-data` survives container recreation and ordinary `docker compose down`. Do not run `docker compose down -v` unless permanent deletion is intended.

Create a logical backup:

```bash
mkdir -p backups
docker compose --env-file deploy/.env.production -f deploy/compose.production.yml \
  exec -T mysql /bin/bash -c \
  'exec mysqldump --single-transaction --routines --triggers -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"' \
  > "backups/animal-poke-$(date +%Y%m%d-%H%M%S).sql"
```

Back up the named volume at the infrastructure/storage layer as well before upgrades.

## 4. Static frontend deployment

```bash
cd frontend
npm ci
npm run build
```

Upload all files under `frontend/dist/` to the static host. Then replace its `config.js` with a copy of `deploy/static-config.example.js` containing the public HTTPS backend URL.

If frontend and API share one origin through a reverse proxy, set `apiBaseUrl` to an empty string and proxy `/api/` to `127.0.0.1:8080`. If they use different origins:

- set `apiBaseUrl` to the backend HTTPS origin;
- set backend `CORS_ALLOWED_ORIGINS` to the frontend origin;
- configure the static host CSP `connect-src` to allow the backend origin.

Never put API keys, JWT secrets, or database credentials in frontend files.

## 5. Upgrade

Set `BACKEND_IMAGE` to an immutable CNB tag, then run:

```bash
docker compose --env-file deploy/.env.production -f deploy/compose.production.yml pull backend
docker compose --env-file deploy/.env.production -f deploy/compose.production.yml up -d backend
```

The default `AUTO_MIGRATE=true` is appropriate for this single-backend Compose deployment. For multiple backend replicas, run `animal-poke-migrate up` as a separate pre-deploy job and set `AUTO_MIGRATE=false`.
