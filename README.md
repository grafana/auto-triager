# Auto Grafana issue triager with Gemini and OpenAI (fine tuned models)

This is a (rather naive) attempt to create an auto triage system for Grafana issues.

## How does it work?

There are two different implementations of the auto triager, one using Gemini with RAG and one using OpenAI fine tuned models.

### Gemini with RAG

The auto-triager uses retrieval-augmented generation (RAG) to generate a long list of historic data
that is later sent to the remote model for analysis.

These are the steps that follows:

- Reads the issue from Github
- Converts the issue title/content to an embedded document via [google text embedding api](https://ai.google.dev/gemini-api/docs/embeddings)
- Queries a pre-built vector database with all the historic issues from Grafana github
- Puts together a prompt using the historic data, the issue content and the possible labels asking the model to classify the issue
- Sends the prompt to the model
- Returns the model's classification JSON output

### OpenAI fine tuned models

The auto-triager has a command to generate a dataset that can later be used to fine tune a model via the UI https://platform.openai.com/finetune/

Two datasets can be generated:

- A dataset for the qualitizer model.
- A dataset for the categorizer model.

The qualitizer model is used to determine if an issue is categorizable or not.
The categorizer model is used to determine the area and type of an issue.

## How to use the fine tuned models

### Requirements

- Go 1.22.3 or higher installed
- [Mage](https://magefile.org/)
- A Github personal access token with read access to public repos in the `GH_TOKEN` env var [1]
- A fine-tuned model. See [openai-finetune.md](./docs/openai-finetune.md)

### Running the triager

```bash
mage -v run:triagerFineTuned [issueId]
```

[1] If you wish the tool to also update the issue with the generated labels you can pass the `-addLabels=true` flag.
Your token must have the necessary permissions to add labels to issues.

Where `[issueId]` is the issue id you want to triage (without the brackets).

## How to use the Gemini with RAG implementation

> [!IMPORTANT]
> You must have the vector database generated before you can use the gemini with RAG implementation.
> See the relevant section on how to generate it.
> Or you can ask a colleague to provide you with a pre-built vector db.

### Requirements

- Go 1.22.3 or higher installed
- [Mage](https://magefile.org/)
- A Github personal access token with read access to public repos in the `GH_TOKEN` env var
- A Google Cloud Platform API key with the text embedding api enabled in the `GEMINI_API_KEY` env var
- A generated vector database with github issues. see [gemini-with-rag.md](./docs/gemini-with-rag.md)

### Running the triager

```bash
mage -v run:triager [issueId]
```

Where `[issueId]` is the issue id you want to triage (without the brackets).

This will also update the vector database in case you have new issues in the sqlitedb.
