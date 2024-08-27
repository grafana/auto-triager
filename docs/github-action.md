# Github action integration

> [!NOTE]
> Currently the github action is using OpenAI fine-tuned models.
> If you want to use the gemini with RAG implementation you must change the [action.yml](../action.yml) file.

## Requirements

- A fine-tuned model (or use the default model hardcoded in the code)
- An openai api key with access to the fine-tuned model

## Usage

To use the action you need to create a workflow file in your repo.

The following working example will attempt to triage issues without the `internal` label and instruct
the action to auto label with the found labels.

```yaml
on:
  issues:
    types: [opened]

jobs:
  check-label:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout repository
        uses: actions/checkout@v3

      - name: Use Check Issue Label action
        id: check_internal
        uses: grafana/auto-triager@main
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
          issue_number: ${{ github.event.issue.number }}
          openai_api_key: ${{ secrets.OPENAI_API_KEY }}
```

## What is it doing?

The code for the action is available in the [action.yml](../action.yml) file.

Interally it simply calls the auto-triager (fine tuned) tool with the passed issue id and instructs it to add the labels to the issue.
