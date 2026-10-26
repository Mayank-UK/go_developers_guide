# Dev client image: Node 22 + npm >= 11 (family trap: Node 22 ships npm
# 10.9.9, which crashes in @npmcli/arborist on client lockfile
# regeneration). Build context: repo root.
#
# Dependency-bake layers (fastapi-family style): lockfile-first install keeps
# the dependency layer cacheable, then the client sources land. Compose
# bind-mounts live source over /app and shadows node_modules with an
# anonymous volume seeded from this layer, so the running container verifies
# against the mounted workspace while a bare `docker run` gets a working dev
# server out of the box.
FROM node:22-bookworm

RUN npm install -g npm@11

WORKDIR /app

# Lockfile-first install keeps the dependency layer cacheable.
COPY client/package.json client/package-lock.json ./
RUN npm ci

COPY client/ ./

EXPOSE 5175
# --host keeps Vite bound to 0.0.0.0 so the published port works from the
# host browser; the port itself (5175) is set by vite.config.ts. Compose
# overrides this with `command: sleep infinity` — the family verification
# path is docker exec at /app.
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
