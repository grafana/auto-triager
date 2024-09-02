const fs = require("fs");
const path = require("path");

// Load commands.json
const jsonPath = path.join(__dirname, "../out", "commands.json");
const commands = JSON.parse(fs.readFileSync(jsonPath, "utf-8"));

// Filter the command objects
const uniqueCommands = [];

commands.forEach((command) => {
  if (command.addToProject) {
    const isDuplicate = uniqueCommands.some(
      (item) =>
        item.name === command.name &&
        item.addToProject.url === command.addToProject.url,
    );

    if (!isDuplicate) {
      uniqueCommands.push(command);
    }
  } else {
    uniqueCommands.push(command);
  }
});

const outFile = path.join(__dirname, "../out", "commands-deduped.json");

// Output the filtered commands
fs.writeFileSync(outFile, JSON.stringify(uniqueCommands, null, 2), "utf-8");
