# Deploy — party.justgui.dev (Docker Swarm + core nginx)

Runs alongside the existing `infra-swarm` stacks, behind `core_nginx` on the
`proxy` overlay network. No secrets live in this repo — they come from
`/opt/infra-swarm/.env`.

## Env vars to add to `/opt/infra-swarm/.env`

```
PARTY_DB_PASSWORD=<random>
PARTY_JWT_SECRET=<random 64 hex>
PARTY_ADMIN_EMAILS=<comma-separated admin emails>

PARTY_YOUTUBE_API_KEY=<youtube data api v3 key>
# SMTP is reused from the existing SMTP_HOST / SMTP_PORT / SMTP_JUSTGUI_* vars.
```

## Steps (on the VPS)

```bash
# 1. Get the code
git clone https://github.com/justgu1/party-manager.git /opt/party-manager
cd /opt/party-manager

# 2. Build the images (SPA + Go embedded)
docker build --target api    -t party-manager-api:latest .
docker build --target worker -t party-manager-worker:latest .

# 3. Install stack + nginx config
cp deploy/party.yml /opt/infra-swarm/stacks/party.yml
cp deploy/party.justgui.dev.conf /opt/infra-swarm/core/nginx/conf.d/

# 4. Issue the TLS cert (bootstrap HTTP conf first so the ACME challenge is served)
cp deploy/party.justgui.dev.bootstrap.conf /opt/infra-swarm/core/nginx/conf.d/party.justgui.dev.conf
docker exec $(docker ps -qf name=core_nginx) nginx -s reload
docker run --rm -v certbot_certs:/etc/letsencrypt -v certbot_www:/var/www/certbot \
  certbot/certbot certonly --webroot --webroot-path=/var/www/certbot \
  --email <CERTBOT_EMAIL> --agree-tos --no-eff-email --non-interactive -d party.justgui.dev
# then swap in the full TLS conf
cp deploy/party.justgui.dev.conf /opt/infra-swarm/core/nginx/conf.d/party.justgui.dev.conf

# 5. Deploy the stack + reload nginx
set -a; . /opt/infra-swarm/.env; set +a
docker stack deploy -c /opt/infra-swarm/stacks/party.yml party
docker exec $(docker ps -qf name=core_nginx) nginx -t && \
docker exec $(docker ps -qf name=core_nginx) nginx -s reload
```

## Updating later

```bash
cd /opt/party-manager && git pull
docker build --target api -t party-manager-api:latest .
docker build --target worker -t party-manager-worker:latest .
docker service update --image party-manager-api:latest    --force party_api
docker service update --image party-manager-worker:latest --force party_worker
```
