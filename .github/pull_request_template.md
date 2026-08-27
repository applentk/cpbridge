## Summary

<!-- Describe what changed and why. Link related issues with "Closes #..." where applicable. -->

## Changes

-

## Verification

- [ ] `rtk pnpm lint`
- [ ] `rtk pnpm check`
- [ ] `cd apps/api && go test -v ./...`
- [ ] `rtk pnpm --filter @cpbridge/web test`
- [ ] `rtk pnpm --filter @cpbridge/extension test`
- [ ] `rtk pnpm --filter @cpbridge/extension build`
- [ ] `rtk pnpm --filter @cpbridge/web build`

## Checklist

- [ ] This PR follows the repository's architecture invariants.
- [ ] No external platform passwords, API keys, or session cookies are stored or sent to the backend.
- [ ] Contest timing and submission validation remain server-authoritative and UTC-based.
- [ ] Contest problem snapshots remain immutable after contest creation.
- [ ] New entity IDs use the required prefixed ID generator.
- [ ] Handlers remain thin; business logic stays in domain services.
- [ ] Database changes include both up and down migrations, if applicable.
- [ ] UI changes include screenshots or a GIF, if applicable.
- [ ] Documentation has been updated, if applicable.

## Screenshots / recordings

<!-- Include before/after visuals for UI changes, or write "N/A". -->

## Notes for reviewers

<!-- Call out trade-offs, follow-up work, migration steps, or anything reviewers should test manually. -->
