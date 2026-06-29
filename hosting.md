# EOC Deployment Guide: Single-Origin on Google Cloud Run + Cloudflare

This guide deploys the **Cerebro Emergency Operations Center (EOC)** as a
**single origin**: one Go container on **Google Cloud Run** serves *both* the
static Astro/Svelte dashboard *and* the API/WebSocket, fronted by **Cloudflare**
for DNS, SSL, and DDoS protection.

> **Why single-origin (not Pages + separate API)?** The frontend talks to the
> backend **same-origin**: the WebSocket URL is `wss://<page-host>/stream`
> ([`web/src/components/Dashboard.svelte`](web/src/components/Dashboard.svelte))
> and REST calls are relative (`fetch("/state")`). Splitting the UI and API onto
> two subdomains would make every live call cross-origin → the dashboard would
> silently fall back to its offline **demo mode**, and would additionally require
> CORS middleware (the API has none). Serving both from one container matches the
> code as written, removes CORS entirely, and removes a whole deploy target.

---

## 1. Deployment Architecture

```mermaid
graph TD
    User([User Browser]) -->|HTTPS + WSS<br/>eoc.yourdomain.com| CF[Cloudflare Proxy / DNS<br/>SSL · DDoS]
    CF -->|proxied CNAME| GCR[Google Cloud Run<br/>single Go container]
    subgraph GCR_SVC [eoc-backend on :PORT]
      Static[Static dashboard<br/>web/dist at /]
      API[API: /state /agents /timeline /events]
      WS[WebSocket: /stream]
    end
    GCR --> GCR_SVC
    GCR_SVC -->|HTTPS| Cerebras[Cerebras Wafer-Scale API<br/>Gemma 4 31B]
```

One container, one origin:
1. **Static dashboard** — the built `web/dist` is served at `/` by the Go server.
2. **API + WebSocket** — `/state`, `/agents`, `/timeline`, `/events`, `/stream`
   on the same host. No CORS needed (same origin).
3. **Cloudflare** — one proxied CNAME → Cloud Run, free SSL/TLS, DDoS.

---

## 2. Prerequisites

- The Go server must (a) serve the static build and (b) **listen on `$PORT`**
  (Cloud Run's contract — it injects `PORT`, default `8080`). See HANDOFF §8
  parcel **P6**; the server reads `PORT` (fallback `8080`) and serves the
  directory in `WEB_DIR` (default `web/dist`) at `/`.
- The container build produces a single image containing both the Go binary and
  `web/dist` (multi-stage `Dockerfile`; see parcel **P8**).

---

## 3. Build the Container (one image, both tiers)

The multi-stage [Dockerfile](Dockerfile) builds the web assets and the Go binary,
then assembles a minimal runtime image. Build it from the project root:

```bash
# Builds web/dist (Node stage) + the eoc binary (Go stage) into one image.
docker build -t REGION-docker.pkg.dev/YOUR_GCP_PROJECT/eoc/eoc-backend:latest .
```

> Use **Artifact Registry** (`REGION-docker.pkg.dev/...`), not the deprecated
> `gcr.io` Container Registry. Create the repo once:
> `gcloud artifacts repositories create eoc --repository-format=docker --location=REGION`.

Push it:

```bash
gcloud auth configure-docker REGION-docker.pkg.dev
docker push REGION-docker.pkg.dev/YOUR_GCP_PROJECT/eoc/eoc-backend:latest
```

---

## 4. Deploy to Cloud Run

```bash
gcloud run deploy eoc-backend \
    --image REGION-docker.pkg.dev/YOUR_GCP_PROJECT/eoc/eoc-backend:latest \
    --platform managed \
    --region REGION \
    --allow-unauthenticated \
    --min-instances 1 \
    --timeout 3600 \
    --set-secrets CEREBRAS_API_KEY=cerebras-api-key:latest \
    --set-env-vars CEREBRAS_MODEL=gemma-4-31b
```

Notes:
- **Do not pass `--port`** unless you have a reason to; Cloud Run injects `$PORT`
  and the server honors it. (The image `EXPOSE`s `8080` as the default.)
- **`--min-instances 1`** avoids cold-start dropping the first WebSocket
  connection during a live demo. (Trade-off: it no longer scales to zero, so it
  costs a little while idle — fine for a demo window; drop back to `0` after.)
- **`--timeout 3600`** raises the request timeout to 60 min so long-lived
  WebSocket streams aren't cut at the 5-min default.
- **Secrets, not env vars, for the API key** — `--set-secrets` pulls from Secret
  Manager so the key isn't visible in `gcloud run services describe` / deploy
  history. Create it once:
  `printf '%s' "$KEY" | gcloud secrets create cerebras-api-key --data-file=-`.
  (If you must, `--set-env-vars CEREBRAS_API_KEY=...` works but is less safe.)
- `gemma-4-31b` is native multimodal — the same model serves text reasoning and
  image perception (`POST /perception`, parcels P2/P5).

Cloud Run prints the service URL (e.g. `https://eoc-backend-xxxx.a.run.app`).
Verify before wiring DNS: open the URL — the dashboard should load and connect to
its own `/stream` in **live** mode (not demo mode).

---

## 5. Cloudflare DNS (single subdomain)

To serve at `eoc.yourdomain.com`:
1. Cloud Run console → service → **Custom domains** can map directly, **or** use
   Cloudflare DNS:
2. Cloudflare Dashboard → your domain → **DNS** → **Add record** → **CNAME**.
3. Name `eoc`, Target = the Cloud Run hostname (without `https://`).
4. **Proxy status: Proxied** (orange cloud) so Cloudflare terminates SSL.
5. **SSL/TLS → Overview → Full (strict)** so Cloudflare↔Cloud Run is encrypted.

That's the only DNS record needed — UI, API, and WSS all ride the one origin.

---

## 6. Verification checklist

- [ ] `docker build .` produces an image containing the binary **and** `web/dist`.
- [ ] `docker run -p 8080:8080 -e PORT=8080 <image>` → dashboard at
      `http://localhost:8080/` loads and connects live to `/stream`.
- [ ] Container also honors a non-default port: `-e PORT=9090 -p 9090:9090` works
      (proves the `$PORT` contract).
- [ ] On Cloud Run, the page loads in **live** mode (HUD shows real metrics, not
      the demo cascade).
- [ ] WSS stays connected through a full scenario replay (no 5-min cutoff).
- [ ] No CORS errors in the browser console (there shouldn't be — same origin).

---

## 7. What this guide intentionally drops vs. a two-origin setup

- **No Cloudflare Pages target** — the Go container serves the UI.
- **No CORS middleware** — same origin means none is required.
- **No `PUBLIC_API_URL` frontend config** — the dashboard's same-origin
  WS/fetch logic works unchanged.

If you ever do want the UI on Cloudflare's CDN separately, that's a larger change
(frontend API-host config + backend CORS + `$PORT`); it is **out of scope** for
this single-origin deployment.
