# stacksmith

Create a service template in the browser; it is stored on the host as
`data/services/<name>/docker-compose.yml`, with additions beside it as
`data/services/<name>/<addition>.yml`.

## Run

```sh
docker compose up -d --build     # API on :36900, interface on :36901
```

Each project also runs on its own:

```sh
docker compose -f api/compose.yaml up -d --build
docker compose -f web/compose.yaml up -d --build
```

The interface is nginx serving the build and proxying `/v1/` to the `api`
service. `data/` is mounted at `/app/data` and is not in git.

## API

```sh
curl localhost:36900/v1/services
curl localhost:36900/v1/services -d '{"image":"node:20-alpine"}'
curl localhost:36900/v1/services/nginx
curl localhost:36900/v1/services/nginx/files/database/mariadb.yml
curl localhost:36900/v1/health
```

`POST` answers `201` with the path, `409` when the service exists and `422` for
invalid input; `PUT` rewrites a service and `DELETE` removes it.

## Develop

```sh
cd api && go run ./cmd/api -root ../data
cd web && npm install && npm run dev
```
