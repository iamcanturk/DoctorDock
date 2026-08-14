# DoctorDock landing page

A self-contained static site for **doctordock.iamcanturk.dev** — one
`index.html` with inline CSS/JS, plus images and the mascot. No build step.

## Files

```
web/
├── index.html            the whole page
├── _headers              Cloudflare Pages caching + security headers
└── assets/               mascot, framed app screenshots
```

## Preview locally

```bash
cd web && python3 -m http.server 8799   # http://localhost:8799
```

## Deploy — Cloudflare Pages

The domain iamcanturk.dev is already on Cloudflare, so the custom domain is
one click.

### Option A — connect the Git repo (auto-deploys on every push)

1. Cloudflare dashboard → **Workers & Pages** → **Create** → **Pages** →
   **Connect to Git** → pick `iamcanturk/DoctorDock`.
2. Build settings: **Framework preset: None**, **Build command: (empty)**,
   **Build output directory: `web`**. Save and Deploy.
3. Project → **Custom domains** → **Set up a domain** → `doctordock.iamcanturk.dev`.
   Cloudflare adds the DNS record automatically.

### Option B — deploy from this machine with Wrangler

```bash
npx wrangler login                                  # one-time, opens a browser
npx wrangler pages deploy web --project-name doctordock
```

Then add the custom domain as in step 3 above.
