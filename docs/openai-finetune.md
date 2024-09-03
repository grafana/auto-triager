# Generate the datasets for the fine-tuned models

If you want to use `auto-triager` with the OpenAI implementation, you may want to fine tune your model.

If you want to use `auto-triager` with the Gemini with Rag implementation, you don't need to follow these steps.

We already have fine-tuned models that are part of the Grafana OpenAI organization and you can use them directly.
To use the existing fine-tuned models, use an API key for the **Grafanalabs experiments exploration** organization.

To generate new datasets to fine tune the models again, follow this guide.

## Requirements

- Go 1.22.3 or higher installed
- [Mage](https://magefile.org/)
- The `GH_TOKEN` environment variable set to a GitHub personal access token with at least read access to public repositories.

  If you want the tool to also update the issue with the generated labels you can pass the `-addLabels=true` option.
  To update issues with labels, your token must also have the permissions to add labels to issues.

  To create the token, refer to [Create a GitHub personal access token](#create-a-GitHub-personal-access-token).

## Prepare the data

You need to modify the fixtures files to adjust the IDs of the issues you want to generate the dataset for.

### Categorizer

- `fixtures/categorizedIds.txt`: list of the ids of the issues that are correctly categorized. Used by the categorizer model.
- `fixtures/categoryLabels.txt`: The area labels of the issues. Used by the categorizer model.
- `fixtures/typeLabels.txt`: The type labels of the issues. Used by the categorizer model.

- `fixtures/categorizedIds.json`: (Optional) JSON file containing the IDs, title, description, and labels of correctly categorized issues. It's added on top of the `categorizedIds.txt` file.

### Qualitizer

- `fixtures/good-issues-ids.txt`: The IDs of the issues that are categorizable. Used by the qualitizer model.
- `fixtures/missingInfoIds.txt`: The IDs of the issues that are missing information. Used by the qualitizer model.

## Generate the datasets

For the categorizer and qualitizer models you need to run the fine-tuner generator tool.
The following command generates the datasets in the `out` directory:

```bash
mage -v run:finetuner categorizer
mage -v run:finetuner qualitizer
```

## Fine tune the models

To fine tune the models you must create a new fine tune job.

1. Browse to <https://platform.openai.com/finetune/>.
1. Create a new fine tune job.
1. Select the base model to fine tune
1. Select the dataset to use from the `out` directory. Use either the categorizer or qualitizer dataset.
1. Choose a name for the job, usually `auto-triager-<qualitizer|categorizer>`
