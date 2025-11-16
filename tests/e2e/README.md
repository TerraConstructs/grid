pnpm init playwright@latest --yes "--" . '--quiet' '--browser=chromium'

> pnpx
> create-playwright . --quiet --browser=chromium

Getting started with writing end-to-end tests with Playwright:
Initializing project in '.'
Installing Playwright Test (npm install --save-dev @playwright/test)…

added 3 packages, and audited 6 packages in 6s

1 package is looking for funding
  run `npm fund` for details

found 0 vulnerabilities
Installing Types (npm install --save-dev @types/node)…

added 3 packages, and audited 9 packages in 2s

1 package is looking for funding
  run `npm fund` for details

found 0 vulnerabilities
Writing playwright.config.ts.
Writing e2e/example.spec.ts.
Writing package.json.
Downloading browsers (npx playwright install chromium)…
Downloading Chromium 141.0.7390.37 (playwright build v1194) from https://cdn.playwright.dev/dbazure/download/playwright/builds/chromium/1194/chromium-mac-arm64.zip
129.7 MiB [====================] 100% 0.0s
Chromium 141.0.7390.37 (playwright build v1194) downloaded to /Users/vincentdesmet/Library/Caches/ms-playwright/chromium-1194
Downloading FFMPEG playwright build v1011 from https://cdn.playwright.dev/dbazure/download/playwright/builds/ffmpeg/1011/ffmpeg-mac-arm64.zip
1 MiB [====================] 100% 0.0s
FFMPEG playwright build v1011 downloaded to /Users/vincentdesmet/Library/Caches/ms-playwright/ffmpeg-1011
Downloading Chromium Headless Shell 141.0.7390.37 (playwright build v1194) from https://cdn.playwright.dev/dbazure/download/playwright/builds/chromium/1194/chromium-headless-shell-mac-arm64.zip
81.7 MiB [====================] 100% 0.0s
Chromium Headless Shell 141.0.7390.37 (playwright build v1194) downloaded to /Users/vincentdesmet/Library/Caches/ms-playwright/chromium_headless_shell-1194
✔ Success! Created a Playwright Test project at /Users/vincentdesmet/tcons/grid

Inside that directory, you can run several commands:

  pnpx playwright test
    Runs the end-to-end tests.

  pnpx playwright test --ui
    Starts the interactive UI mode.

  pnpx playwright test --project=chromium
    Runs the tests only on Desktop Chrome.

  pnpx playwright test example
    Runs the tests in a specific file.

  pnpx playwright test --debug
    Runs the tests in debug mode.

  pnpx playwright codegen
    Auto generate tests with Codegen.

We suggest that you begin by typing:

    pnpx playwright test

And check out the following files:
  - ./tests/e2e/example.spec.ts - Example end-to-end test
  - ./playwright.config.ts - Playwright Test configuration

Visit https://playwright.dev/docs/intro for more information. ✨

Happy hacking! 🎭
