# Frontend browser tests

The smoke suite starts Vite automatically and intercepts gateway requests in
the browser, so a running backend is not required.

The suite covers anonymous catalog browsing, authentication with protected
navigation, library loading, and the creator workflow for discovering licensed
provider audio and starting an external import for a draft release track.

Install the Playwright browser once and run the suite:

```bash
npx playwright install chromium
npm run test:e2e
```

To reuse a system Chromium installation:

```bash
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/chromium npm run test:e2e
```

Use `npm run test:e2e:ui` for interactive debugging. Failed tests retain a
trace and screenshot in the ignored `test-results` directory.

Pull requests that change the frontend run linting, type checking, unit tests,
the production build, and this smoke suite through the `Frontend checks`
GitHub Actions workflow. A failed browser run uploads its HTML report for seven
days.

The `creatorPublication.spec.ts` scenario uses a **stateful mocked gateway** shared
by independent creator, moderator, and listener browser contexts. It covers
account verification, draft, cover/audio ingest, rights, readiness, submission,
independent review, publication, delayed Catalog visibility, and playback requests.
The mock does not run FFmpeg, Kafka, PostgreSQL, or MinIO; actual service-to-service
publication and playback still require an isolated system smoke test.
