# cpbridge Deployment

This deployment uses Vercel for the SvelteKit web app and Render for the Go API,
PostgreSQL, and Redis.

## Domains

- Web: `https://cpbridge.applentk.com`
- API: `https://api.cpbridge.applentk.com`

The web app keeps using `/api/...`; `apps/web/vercel.json` rewrites those requests
to the Render API domain.

## Vercel

Create a Vercel project from the repository with `apps/web` as the Root Directory.
Use the default pnpm install and build settings. The app uses the Vercel SvelteKit
adapter and includes the API rewrite configuration.

Add `cpbridge.applentk.com` as the Vercel custom domain and configure the DNS record
requested by Vercel.

## Render

Create a Blueprint from `render.yaml`. It provisions:

- A Singapore Go web service at `api.cpbridge.applentk.com`
- A Singapore Render Key Value instance for Asynq/Redis
- A Singapore PostgreSQL 16 database

The API service starts the HTTP server and the Asynq worker in one persistent
process. Keep it at one instance until worker coordination and horizontal scaling
are explicitly designed.

During the initial Blueprint setup, provide `INITIAL_ADMIN_EMAIL` if the first
registered account should be promoted to admin. Do not commit secrets.

## DNS

Configure the following records at the domain registrar:

1. The Vercel record for `cpbridge.applentk.com`.
2. The Render custom-domain record for `api.cpbridge.applentk.com`.

The exact record values are provider-specific and are shown by Vercel and Render
when each custom domain is added.
