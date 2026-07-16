<!-- CUSTOM: Safe operating guide for isolated secondary-development stacks. -->

# Custom and Test Docker Environments

`docker-compose.dev.yml` supports two isolated environments from the same source tree:

| Environment | Project | Containers | Host address | Data prefix |
| --- | --- | --- | --- | --- |
| custom | `sub2api-custom` | `sub2api-custom*` | `127.0.0.1:18080` | `custom_` |
| test | `sub2api-test` | `sub2api-test*` | `127.0.0.1:18081` | `test_` |

The official production deployment is not part of either project.

## Prepare environment files

```bash
cp deploy/.env.custom.example deploy/.env.custom
cp deploy/.env.test.example deploy/.env.test
chmod 600 deploy/.env.custom deploy/.env.test
```

Replace every `replace_with_...` value. Do not copy secrets from production.

## Validate before starting

```bash
docker compose --env-file deploy/.env.custom -f deploy/docker-compose.dev.yml config
docker compose --env-file deploy/.env.test -f deploy/docker-compose.dev.yml config
```

Confirm the rendered project name, container names, host port, bind paths, and the internal `postgres` and `redis` hosts before starting either environment.

## Build and start

```bash
docker compose --env-file deploy/.env.custom -f deploy/docker-compose.dev.yml up -d --build
docker compose --env-file deploy/.env.test -f deploy/docker-compose.dev.yml up -d --build
```

## Inspect an environment

```bash
docker compose --env-file deploy/.env.custom -f deploy/docker-compose.dev.yml ps
docker compose --env-file deploy/.env.custom -f deploy/docker-compose.dev.yml logs -f sub2api
```

Replace `.env.custom` with `.env.test` for the test environment.

## Stop an environment

```bash
docker compose --env-file deploy/.env.custom -f deploy/docker-compose.dev.yml down
docker compose --env-file deploy/.env.test -f deploy/docker-compose.dev.yml down
```

Do not omit `--env-file` or `-f`. Do not use `down -v`, `docker volume prune`, or `docker system prune` as routine commands on a host that also runs production.
