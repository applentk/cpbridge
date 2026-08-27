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

The Blueprint currently uses Render's free plans for all three resources. This is
appropriate for a temporary demo environment only: the API can spin down after
inactivity, the Redis instance is non-persistent, and the free PostgreSQL database
is subject to Render's free-tier lifecycle limits. The API service starts the HTTP
server and the Asynq worker in one process, so queued verdict polling is not
reliable while the free API is asleep.

For a production deployment, change the API and Redis plans to `starter` and the
PostgreSQL plan to `basic-256mb` or larger, and restore Redis persistence.

During the initial Blueprint setup, provide `INITIAL_ADMIN_EMAIL` if the first
registered account should be promoted to admin. Do not commit secrets.

## DNS

Configure the following records at the domain registrar:

1. The Vercel record for `cpbridge.applentk.com`.
2. The Render custom-domain record for `api.cpbridge.applentk.com`.

The exact record values are provider-specific and are shown by Vercel and Render
when each custom domain is added.

## CI/CD

GitHub Actions runs the full verification suite for every pull request and every
push to `main` through `.github/workflows/ci.yml`. The checks cover ESLint,
workspace type-checking, Go API tests, extension tests/builds, Playwright web
tests, and the production web build.

Production deployment is intentionally a separate, manually dispatched workflow
in `.github/workflows/deploy.yml`. Configure a GitHub `production` environment
with required reviewers and add these environment secrets:

- `RENDER_DEPLOY_HOOK_URL`: a Render deploy hook for `cpbridge-api`.
- `VERCEL_DEPLOY_HOOK_URL`: a Vercel deploy hook for the web project rooted at `apps/web`.

After CI passes, run the Deploy workflow and select which provider(s) to trigger.
The provider hooks perform the actual builds and deployments; secrets stay in
GitHub and are never committed to the repository.
