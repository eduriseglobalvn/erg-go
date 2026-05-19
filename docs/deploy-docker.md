# Docker Deploy

## Self-host from cloned source

Use this flow when the VPS clones the repository and builds the image locally:

```bash
git clone <repo-url> erg-go
cd erg-go
sh scripts/deploy-prod.sh
```

The compose file starts `erg-server`, `postgres`, `mongodb`, and `redis` on the same Docker network.
On the first run, the script creates `.env.production` from `.env.production.example` and stops if placeholder secrets are still present.

## Build locally and push

```bash
docker build -t your-dockerhub-user/erg-go:latest .
docker push your-dockerhub-user/erg-go:latest
```

On the VPS, set `ERG_IMAGE=your-dockerhub-user/erg-go:latest` in `.env.production`, then:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml pull
docker compose --env-file .env.production -f docker-compose.prod.yml up -d
```

## Run database migrations and seed data

Run migrations explicitly before or after updating the service image:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml run --rm --entrypoint db-migrate erg-server
```

Seed default LMS/Hoclieu data when preparing a new environment:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml run --rm --entrypoint lms-seed erg-server
docker compose --env-file .env.production -f docker-compose.prod.yml run --rm --entrypoint hoclieu-seed erg-server
```

## Update deployment

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml pull
docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build
docker image prune -f
```

## Useful checks

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml ps
docker compose --env-file .env.production -f docker-compose.prod.yml logs -f erg-server
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/ready
```
