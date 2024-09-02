const fs = require("fs");
const path = require("path");

const file = path.resolve(__dirname, "labels.json");

fs.readFile(file, "utf8", (err, data) => {
  if (err) {
    console.error("Error reading the file:", err);
    return;
  }

  const labels = JSON.parse(data);
  const transformed = [];

  labels.forEach(({ label, projects }) => {
    projects.forEach((project) => {
      transformed.push({
        type: "label",
        name: label,
        action: "addToProject",
        addToProject: {
          url: project,
        },
      });
    });
  });

  // write to commands.json
  fs.writeFile(
    path.resolve(__dirname, "../out", "commands.json"),
    JSON.stringify(transformed, null, 2),
    (err) => {
      if (err) {
        console.error("Error writing to commands.json:", err);
        return;
      }
      console.log("Commands.json written successfully.");
    },
  );
});
