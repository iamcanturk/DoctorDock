# DoctorDock landing page

A self-contained static site for **[doctordock.iamcanturk.dev](https://doctordock.iamcanturk.dev)**
— one `index.html` with inline CSS/JS, plus images, the mascot, and the SEO/GEO
files. No build step.

## Files

```
web/
├── index.html            the whole page (meta, JSON-LD, FAQ, inline CSS/JS)
├── _headers              caching + security headers
├── robots.txt            crawler policy + sitemap pointer
├── sitemap.xml           the single page, for search engines
├── llms.txt              a plain-text brief for AI answer engines (GEO)
├── README.md             this file
└── assets/               mascot, framed app screenshots, the share/OG card
```

The deploy config lives at the repo root: [`wrangler.jsonc`](../wrangler.jsonc)
serves `web/` as a Cloudflare **assets-only Worker** (no Worker code, just the
static files).

## Preview locally

```bash
cd web && python3 -m http.server 8799   # http://localhost:8799
```

## Deploy — Cloudflare (Workers static assets)

The domain `iamcanturk.dev` is already on Cloudflare, so the custom domain is
one click. This site is deployed as an assets-only Worker via `wrangler.jsonc`.

### How it's set up now — Git-connected (auto-deploys on every push)

1. Cloudflare dashboard → **Workers & Pages** → **Create** → **Import a
   repository** → pick `iamcanturk/DoctorDock`.
2. Build settings: **Build command: (empty)**, **Deploy command:
   `npx wrangler deploy`**. Wrangler reads `wrangler.jsonc` and uploads `web/`.
3. Project → **Settings → Domains & Routes → Add → Custom Domain** →
   `doctordock.iamcanturk.dev`. Cloudflare adds the proxied DNS record
   automatically and issues the certificate.

Every push to `main` now redeploys the landing page — no build step, no action
minutes.

### Deploy from this machine instead

```bash
npx wrangler login       # one-time, opens a browser
npx wrangler deploy      # reads wrangler.jsonc, uploads web/
```
