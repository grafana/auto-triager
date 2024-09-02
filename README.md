# Automatically triage Grafana issues with Gemini and OpenAI (fine tuned models)

`auto-triager` automatically triages Grafana issues using OpenAI GPT or Google Gemini.

It uses the categorizer model to determine the area and type of an issue.

## GitHub action integration

You can use the tool with GitHub Actions to automatically triage issues.

For more information, refer to [GitHub Action integration](./docs/github-action.md)

## Use the OpenAI fine tuned models

To use the OpenAI fine-tuned models,

### Requirements

- Go 1.22.3 or higher installed
- [Mage](https://magefile.org/)
- The `GH_TOKEN` environment variable set to a GitHub personal access token with at least read access to public repositories.

  If you want the tool to also update the issue with the generated labels you can pass the `-addLabels=true` flag.
  To update issues with labels, your token must also have the permissions to add labels to issues.

- A fine-tuned model. For more information, refer to [Generate the datasets for the fine-tuned models](./docs/openai-finetune.md).

### Run `auto-triager`

To run `auto-triager`, use the following command:

```bash
mage -v run:triagerFineTuned <ISSUE ID>
```

Where _`ISSUE ID`_ is the issue ID you want to triage.

## How to use the Gemini with RAG implementation

> [!IMPORTANT]
> You must have the vector database generated before you can use the Gemini with RAG implementation.
> See the relevant section on how to generate it.
> Or you can ask a colleague to provide you with a pre-built vector db.

### Requirements

- Go 1.22.3 or higher installed
- [Mage](https://magefile.org/)
- The `GH_TOKEN` environment variable set to a GitHub personal access token with at least read access to public repositories.

  If you want the tool to also update the issue with the generated labels you can pass the `-addLabels=true` flag.
  To update issues with labels, your token must also have the permissions to add labels to issues.

- The `GEMINI_API_KEY` environment variable set to a Google Cloud Platform API key with the text embedding API enabled.
- A vector database populated with GitHub issues.
  To populate the database, refer to [Populate the vector database](./docs/gemini-rag.md)

### Run `auto-triager`

To run `auto-triager`, use the following command:

```bash
mage -v run:triagerFineTuned <ISSUE ID>
```

Where _`ISSUE ID`_ is the issue ID you want to triage.

This also updates the vector database in case you have new issues in the SQLite database.

## How does it work?

There are two different implementations of the auto-triager, one using Gemini with RAG, and one using OpenAI fine-tuned models.
The OpenAI implementation doesn't have to be fine tuned but it works better if it is.

### Gemini with RAG

`auto-triager` uses retrieval-augmented generation (RAG) to generate a long list of historic data that's later sent to the remote model for analysis.

These are the steps that follows:

1. Read the issue from GitHub.
1. Convert the issue title and content to an embedded document using the [Google text embedding API](https://ai.google.dev/gemini-api/docs/embeddings).
1. Query a pre-built vector database with all the historic issues from Grafana GitHub.
1. Create a prompt using the historic data, the issue content, and the possible labels asking the model to classify the issue.
1. Send the prompt to the model.
1. Return the model's classification JSON output.

### OpenAI fine-tuned models

`auto-triager` has a command to generate a dataset that you can use later to fine tune a model in the [UI](https://platform.openai.com/finetune/).

You can generate two datasets:

- A dataset for the qualitizer model.
- A dataset for the categorizer model.

The qualitizer model is used to determine if an issue is categorizable or not.
