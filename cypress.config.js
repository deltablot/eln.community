const { defineConfig } = require('cypress')

module.exports = {
  allowCypressEnv: false,
  fixturesFolder: 'tests/cypress/fixtures',
  screenshotsFolder: 'tests/cypress/screenshots',
  videosFolder: 'tests/cypress/videos',
  video: false,
  viewportHeight: 900,
  viewportWidth: 1440,
  e2e: {
    setupNodeEvents(on, config) {
      // implement node event listeners here
    },
    baseUrl: 'http://localhost:8080',
    supportFile: 'tests/cypress/support/e2e.{js,jsx,ts,tsx}',
    specPattern: 'tests/cypress/integration/**/*.cy.{js,jsx,ts,tsx}',
  },
  defaultCommandTimeout: 4000,
  requestTimeout: 5000,
  responseTimeout: 30000,
  taskTimeout: 60000,
};
