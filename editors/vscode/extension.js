// VSCode extension entry point
// Provides C0 language support via LSP integration

const vscode = require("vscode");

function activate(context) {
  // Register the C0 language
  const c0Selector = { scheme: "file", language: "c0" };

  // Start the LSP client
  const serverOptions = {
    command: "c0",
    args: ["lsp"],
    options: {
      cwd: vscode.workspace.rootPath,
    },
  };

  const clientOptions = {
    documentSelector: [c0Selector],
    synchronize: {
      fileEvents: vscode.workspace.createFileSystemWatcher("**/*.c0"),
    },
  };

  const lspClient = new vscode.LanguageClient(
    "c0",
    "C0 Language Server",
    serverOptions,
    clientOptions,
  );

  context.subscriptions.push(lspClient.start());
  console.log("C0 extension activated");
}

function deactivate() {}

module.exports = {
  activate,
  deactivate,
};
