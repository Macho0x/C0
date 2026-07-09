// VSCode extension entry point
// Provides Goop language support via LSP integration

const vscode = require("vscode");

function activate(context) {
  const goopSelector = { scheme: "file", language: "goop" };

  const serverOptions = {
    command: "goop",
    args: ["lsp"],
    options: {
      cwd: vscode.workspace.rootPath,
    },
  };

  const clientOptions = {
    documentSelector: [goopSelector],
    synchronize: {
      fileEvents: vscode.workspace.createFileSystemWatcher("**/*.goop"),
    },
  };

  const lspClient = new vscode.LanguageClient(
    "goop",
    "Goop Language Server",
    serverOptions,
    clientOptions,
  );

  context.subscriptions.push(lspClient.start());
  console.log("Goop extension activated");
}

function deactivate() {}

module.exports = {
  activate,
  deactivate,
};
