// VSCode extension entry point — syntax highlighting + LSP via goop lsp

const vscode = require("vscode");
const { LanguageClient, TransportKind } = require("vscode-languageclient/node");

/** @type {LanguageClient | undefined} */
let client;

/**
 * @param {vscode.ExtensionContext} context
 */
function activate(context) {
  const serverOptions = {
    command: "goop",
    args: ["lsp"],
    transport: TransportKind.stdio,
    options: {
      cwd: vscode.workspace.workspaceFolders?.[0]?.uri.fsPath,
    },
  };

  const clientOptions = {
    documentSelector: [{ scheme: "file", language: "goop" }],
    synchronize: {
      fileEvents: vscode.workspace.createFileSystemWatcher("**/*.goop"),
    },
  };

  client = new LanguageClient("goop", "Goop Language Server", serverOptions, clientOptions);
  context.subscriptions.push(client.start());
}

function deactivate() {
  if (!client) {
    return undefined;
  }
  return client.stop();
}

module.exports = {
  activate,
  deactivate,
};
