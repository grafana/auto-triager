# Populate the vector database

If you want to use `auto-triager` with the Gemini with Rag implementation, you must first populate a vector database with GitHub issues.

If you want to use `auto-triager` with the OpenAI implementation, you don't need to follow these steps.

## Requirements

- Go 1.22.3 or higher installed
- [Mage](https://magefile.org/)
- The `GH_TOKEN` environment variable set to a GitHub personal access token with at least read access to public repositories.

  If you want the tool to also update the issue with the generated labels you can pass the `-addLabels=true` option.
  To update issues with labels, your token must also have the permissions to add labels to issues.

  To create the token, refer to [Create a GitHub personal access token](#create-a-GitHub-personal-access-token).

- The `GEMINI_API_KEY` environment variable set to a Google Cloud Platform API key with the text embedding API enabled.

## Create a GitHub personal access token

To create a GitHub personal access token with the necessary permissions:

1. Browse to the [Fine-grained personal access tokens GitHub settings page](https://github.com/settings/tokens?type=beta)
1. Click **Generate new token**.
1. Fill out the **Name**, **Expiration**, and **Description** fields.
1. Set **Repository Access** to **Public repositories (read only)**
1. Click **Generate token**

To use the token, export the it as the value for the `GH_TOKEN` environment variable.
For example:

```bash
export GH_TOKEN=<TOKEN>
```

## Scrape all the issues from the Grafana GitHub organization

To scrape all the issues from the Grafana GitHub organization, a scraper is included in the tool.
To run the issue scraper:

1. Clone this repository.
1. Delete the file `github-data.sqlite` if it exists.
1. If you've exported the `GH_TOKEN` environment variable, run `mage run:scrapper`.
   Otherwise, to run the command with your token, run `GH_TOKEN=<TOKEN> mage run:scrapper`.
1. Wait for the process to complete.
   It can take a long time.

If the scraper runs without error, it creates a file called `github-data.sqlite` in your current directory.
It should be around 14 GB in size.
You can browse the database with an SQLite database viewer like [DB Browser for SQLite (DB4S)](https://sqlitebrowser.org/).

### Update the vector database

To update the vector database you need to run `auto-triager` with the `-updateVectors=true` option.
Mage has a build target that already includes that option.

- Ensure the `github-data.sqlite` file exists and is populated.
  To populate the database file, refer to [Scrape all the issues from the Grafana GitHub organization](#scrape-all-the-issues-from-the-grafana-github-organization)
- Set the `GEMINI_API_KEY` environment variable with your Gemini API key.
- Run `mage run:triager <ISSUE ID>`.

Or you can run `auto-triager` directly:

Run with `-h` to see the available options.

```bash
go run ./pkg/cmd/triager/triager.go -h
```

Example of running `auto-triager` directly:

```bash
go run ./pkg/cmd/triager/triager.go -issueId 89449 -updateVectors=true
```

Running `auto-triager` with `-updateVectors=true` only updates new entries in the vector database.
