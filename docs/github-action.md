# GitHub Action integration

> [!NOTE]
> The GitHub action uses OpenAI fine-tuned models.
> If you want to use the Gemini with RAG implementation you must change the [action.yml](../action.yml) file.

## Requirements

- A GitHub token with permissions to read and write issue metadata.
- A fine-tuned model (or use the default model set in the code)
- An OpenAI API key with access to the fine-tuned model

## Usage

To use the action you need to create a workflow file in your repository.

The following working example triages any issues that don't have the `internal` label and adds the area and type labels to the issue.
To use the example, you must first create the following repository secrets:

- `GITHUB_TOKEN`: A token permissions to read and write issue metadata.
- `OPENAI_API_KEY`: An OpenAI API key with access to the fine-tuned model.

```yaml
on:
  issues:
    types: [opened]

jobs:
  check-label:
    runs-on: ubuntu-latest

    steps:
      - name: Check out repository
        uses: actions/checkout@v4

      - name: Use Check Issue Label action
        id: check_internal
        uses: grafana/auto-triager@main
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
          issue_number: ${{ github.event.issue.number }}
          openai_api_key: ${{ secrets.OPENAI_API_KEY }}
```

The code for action is available in the [action.yml](../action.yml) file.
