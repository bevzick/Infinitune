# Frontend browser tests

The smoke suite starts Vite automatically and intercepts gateway requests in
the browser, so a running backend is not required.

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
