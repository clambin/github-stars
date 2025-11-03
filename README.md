# github-stars
[![release](https://img.shields.io/github/v/tag/clambin/github-stars?color=green&label=release&style=plastic)](https://github.com/clambin/github-stars/releases)
[![codecov](https://img.shields.io/codecov/c/gh/clambin/github-stars?style=plastic)](https://app.codecov.io/gh/clambin/github-stars)
[![build](https://github.com/clambin/github-stars/workflows/build/badge.svg)](https://github.com/clambin/github-stars/actions)
[![go report card](https://goreportcard.com/badge/github.com/clambin/github-stars)](https://goreportcard.com/report/github.com/clambin/github-stars)
[![license](https://img.shields.io/github/license/clambin/github-stars?style=plastic)](LICENSE.md)

Receives Star events from GitHub and posts them to Slack.

## Configuring a GitHub App
github-stars is implemented as a GitHub App: rather than the server polling for new stars, a GitHub App will forward
the events to a webhook when they occur.

To configure a GitHub app:

- Go to your GitHub account or organization settings.
- Navigate to Settings > Developer settings > GitHub Apps.
- Click New GitHub App.
- Fill in the details:
  - GitHub App Name: Choose a name for your app.
  - Homepage URL: Enter a valid URL (e.g., your app’s website or GitHub profile).
  - Webhook URL: Add the endpoint where your app will receive event notifications.
  - Webhook secret: Add a secure secret string
- In the Repository Permissions section, grant the following permission:
  - Metadata: Read-only (required to identify repositories).
- In the Subscribe to Events section, check the Star event.
- Save your app.
- Install the app to your account. You can give access to all repositories, or a subset.


## Configuring github-stars

github-stars supports the following commandline options:

```aiignore
  -archived
        include archived repositories
  -directory string
        database directory (default ".")
  -github.token string
        GitHub API token
  -github.webhook.addr string
        address to listen on for GitHub webhook calls (default ":8080")
  -github.webhook.secret string
        secret to verify GitHub webhook calls
  -log.format string
        log format (default "text")
  -log.level string
        log level (default "info")
  -prom.addr string
        prometheus listen address (default ":9100")
  -prom.path string
        prometheus path (default "/metrics")
  -slack.webhook string
        Slack webhook URL to post messages to
  -user string
        user to scan for repositories
```

At a minimum, you will need to configure:

- github.token: a GitHub personal access token granting access to your repositories.
- github.webhook.secret: the Webhook secret configured in the GitHub app.
- user: your GitHub account name.
- slack.webhook: the Slack webHook to use to post to your Slack workspace / channel.

## Authors

* **Christophe Lambin**

## License

This project is licensed under the [MIT](LICENSE.md) license.