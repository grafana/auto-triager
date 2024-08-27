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

## Generating the datasets for the fine tuned models

> [!NOTE]
> You don't need to do this if you want to use the gemini with RAG implementation.
> or use the existing fine-tuned models.

If you want to generate new datasets to finetune the models again you can follow the steps below.

We already have fine-tuned models that are part of the grafana openai organization and you can use them directly.

Make sure you are using an API key for the "Grafanalabs experiments exploration" organization.

To generate the datasets you need to run the fine-tuner generator tool. It is easiest to run it with the `mage` tool.

### Requirements

- Go 1.22.3 or higher installed
- [Mage](https://magefile.org/)
- A Github personal access token with read access to public repos in the `GH_TOKEN` env var

### Prepare the data

You need to modify the fixtures files to adjust the ids of the issues you want to generate the dataset for.

- `fixtures/categorizedIds.txt`: The ids of the issues that are correctly categorized. (used by the categorizer model)
- `fixtures/areaLabels.txt`: The area labels of the issues. (used by the categorizer model)
- `fixtures/typeLabels.txt`: The type labels of the issues. (used by the categorizer model)
- `fixtures/good-issues-ids.txt`: The ids of the issues that are consideredd categorizable. (used by the qualitizer model)
- `fixtures/missingInfoIds.txt`: The ids of the issues that are missing information. (used by the qualitizer model)

### Generate the datasets

```bash
mage -v run:finetuner qualitizer
mage -v run:finetuner categorizer
```

This will generate the datasets in the `out` folder.

### Fine tune the models

To fine tune the models you need to go to https://platform.openai.com/finetune/ and create a new fine tune job.

- Select the base model to fine tune
- Select the dataset to use from the out folder (categorizer or qualitizer)
- Put a name for the job: Usually auto-triager-[qualitizer|categorizer]

## Populating the vector database

> [!NOTE]
> This is only needed if you want to use the gemini with RAG implementation.
> If you want to use the fine tuned models you can skip this step.

### Requirements

- Go 1.22.3 or higher installed
- [Mage](https://magefile.org/)
- A Github personal access token with read access to public repos in the `GH_TOKEN` env var
- A Google Cloud Platform API key with the text embedding api enabled

### Scraping all the issues from Grafana github

To scrap all the issues from github a scrapper is included in the tool.

#### Create a personal github token

You will first have to create a personal github token with "read" access to public repos.

- Go to [https://github.com/settings/tokens?type=beta](https://github.com/settings/tokens?type=beta)
- Generate "new token". Give it a name, expiration date (up to you). Repository Access: Public repositories (read only). Save it.
- Export this token in your terminal as `GH_TOKEN`. e.g. : `export GH_TOKEN=YOUR_TOKEN` or pass it when you run the utility

#### Run the issue scrapper

- Clone this repository
- Delete the file `github-data.sqlite` if it exists
- Run `mage run:scrapper`. You can also run it as `GH_TOKEN=YOUR_TOKEN mage run:scrapper` to pass your token.
- Wait.... wait... wait... Maybe get a coffee, or two.

If no errors, you should see a file called `github-data.sqlite` in the current directory. It should be
around 14GB. You can see the db with a sqlite db viewer like [sqlitebrowser](https://sqlitebrowser.org/)

#### Update the vector database.

To update the vector db you need to run the triager tool with the `-updateVectors=true` flag.
Mage has a build target already including that flag.

- Make sure your github-data.sqlite file exists and it is populated.
- Make sure you have your `GEMINI_API_KEY` env var set or pass it to the command..
- run `mage run:triager [issueId]` e.g.: `mage run:triager 89449`

Alternativelely you can run the triager directly:

Run with `-h` to see the available flags

```bash
go run ./pkg/cmd/triager/triager.go -h
```

Example of running the triager directly:

```bash
go run ./pkg/cmd/triager/triager.go -issueId 89449 -updateVectors=true
```

Running with `-updateVectors=trur` will only update new entries in the sqlitedb.

## How to use the fine tuned models

> [!IMPORTANT]
> Make sure you are using the API key for the "Grafanalabs experiments exploration" organization.

### Requirements

- Go 1.22.3 or higher installed
- [Mage](https://magefile.org/)
- A Github personal access token with read access to public repos in the `GH_TOKEN` env var

### Running the triager

```bash
mage -v run:triagerFineTuned [issueId]
```

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

### Running the triager

```bash
mage -v run:triager [issueId]
```

Where `[issueId]` is the issue id you want to triage (without the brackets).

This will also update the vector database in case you have new issues in the sqlitedb.
