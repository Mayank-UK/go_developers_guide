# Dev client image: Node 22 + npm >= 11 (family trap: Node 22 ships npm
# 10.9.9, which crashes in @npmcli/arborist on client lockfile
# regeneration). Build context: repo root.
#
# client/package.json arrives in Phase 0.C. Until then this image is the
# toolchain shell that compose keeps alive (sleep infinity) for
# docker exec; at 0.C the dependency-bake layers land here, fastapi-family
# style:
#   COPY client/package.json client/package-lock.json ./
#   RUN npm ci
#   COPY client/ ./
FROM node:22-bookworm

RUN npm install -g npm@11

WORKDIR /app

EXPOSE 5175
# The Vite dev-server command arrives with client/ (0.C); compose drives the
# container with `command: sleep infinity` until then (and beyond — the
# family verification path is docker exec at /app).
CMD ["sleep", "infinity"]
