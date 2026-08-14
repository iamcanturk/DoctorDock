# DoctorDock landing page

A single self-contained static page for **doctordock.iamcanturk.dev**. No build
step, no dependencies — `index.html` plus one image.

## Files

```
web/
├── index.html            the whole page (HTML + CSS + a little JS, inline)
└── assets/
    └── health-card.png   the hero image (the app's shareable card)
```

## Preview locally

```bash
cd web && python3 -m http.server 8799
# open http://localhost:8799
```

## Deploy

It is a static folder, so any static host works. Point the host at `web/` (or
copy its contents to the web root) and map the domain.

- **Cloudflare Pages / Netlify / Vercel** — set the project root or publish
  directory to `web`, no build command.
- **GitHub Pages** — publish the `web/` folder.
- **Own server** — copy `web/` to the document root.

## Updating the hero image

The hero is the app's own shareable card. Regenerate it from the macOS app:

```bash
cd app/macos
./scripts/build-app.sh
./build/DoctorDock.app/Contents/MacOS/DoctorDock --render /tmp/dd
cp /tmp/dd/share-card.png ../../web/assets/health-card.png
```
