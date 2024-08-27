# Generating the datasets for the fine tuned models

> [!NOTE]
> You don't need to do this if you want to use the gemini with RAG implementation.
> or use the existing fine-tuned models.

If you want to generate new datasets to finetune the models again you can follow the steps below.

We already have fine-tuned models that are part of the grafana openai organization and you can use them directly.

Make sure you are using an API key for the "Grafanalabs experiments exploration" organization.

To generate the datasets you need to run the fine-tuner generator tool. It is easiest to run it with the `mage` tool.

## Requirements

- Go 1.22.3 or higher installed
- [Mage](https://magefile.org/)
- A Github personal access token with read access to public repos in the `GH_TOKEN` env var

## Prepare the data

You need to modify the fixtures files to adjust the ids of the issues you want to generate the dataset for.

### Categorizer

- `fixtures/categorizedIds.txt`: list of the ids of the issues that are correctly categorized. (used by the categorizer model)
- `fixtures/areaLabels.txt`: The area labels of the issues. (used by the categorizer model)
- `fixtures/typeLabels.txt`: The type labels of the issues. (used by the categorizer model)

- [optional] `fixtures/categorizedIds.json`: Json file containing the ids, title, description and labels of the issues that are correctly categorized. Will be added on top of the `categorizedIds.txt` file.

### Qualitizer

- `fixtures/good-issues-ids.txt`: The ids of the issues that are considered "good" and thus categorizable. (used by the qualitizer model)
- `fixtures/missingInfoIds.txt`: The ids of the issues that are missing information. (used by the qualitizer model)

## Generate the datasets

For the categorizer and qualitizer models you need to run the fine-tuner generator tool.

```bash
mage -v run:finetuner categorizer
mage -v run:finetuner qualitizer
```

This will generate the datasets in the `out` folder.

## Fine tune the models

To fine tune the models you need to go to https://platform.openai.com/finetune/ and create a new fine tune job.

- Select the base model to fine tune
- Select the dataset to use from the out folder (categorizer or qualitizer)
- Put a name for the job: Usually auto-triager-[qualitizer|categorizer]
