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
