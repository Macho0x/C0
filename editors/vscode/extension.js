// VSCode extension — syntax highlighting + LSP via `goop lsp`

const fs = require("fs");
const path = require("path");
const vscode = require("vscode");
const { LanguageClient, TransportKind } = require("vscode-languageclient/node");

/** @type {LanguageClient | undefined} */
let client;

/**
 * @param {vscode.WorkspaceConfiguration} config
 * @param {string | undefined} workspaceRoot
 * @returns {string}
 */
function resolveGoopPath(config, workspaceRoot) {
  const configured = config.get("path", "").trim();
  if (configured) {
    if (path.isAbsolute(configured)) {
      return configured;
    }
    if (workspaceRoot) {
      return path.join(workspaceRoot, configured);
    }
    return configured;
  }

  const candidates = [];
  if (workspaceRoot) {
    candidates.push(
      path.join(workspaceRoot, "goop"),
      path.join(workspaceRoot, "bin", "goop"),
      path.join(workspaceRoot, "src", "goop"),
    );
    const parent = path.dirname(workspaceRoot);
    candidates.push(path.join(parent, "goop"));
  }

  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) {
      return candidate;
    }
  }

  return "goop";
}

/**
 * @param {string} goopPath
 * @returns {boolean}
 */
function goopExists(goopPath) {
  if (goopPath === "goop") {
    return true;
  }
  try {
    return fs.existsSync(goopPath);
  } catch {
    return false;
  }
}

/**
 * @param {vscode.ExtensionContext} context
 */
function activate(context) {
  const config = vscode.workspace.getConfiguration("goop");
  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
  const goopPath = resolveGoopPath(config, workspaceRoot);

  if (!goopExists(goopPath)) {
    const msg =
      `Goop compiler not found at "${goopPath}". ` +
      "Build with `cd src && go build -o ../goop ./cmd/goop` " +
      "or set `goop.path` in settings.";
    vscode.window.showWarningMessage(msg);
  }

  const serverOptions = {
    command: goopPath,
    args: ["lsp"],
    transport: TransportKind.stdio,
    options: workspaceRoot ? { cwd: workspaceRoot } : {},
  };

  const clientOptions = {
    documentSelector: [{ scheme: "file", language: "goop" }],
    synchronize: {
      fileEvents: vscode.workspace.createFileSystemWatcher("**/*.goop"),
    },
  };

  client = new LanguageClient("goop", "Goop Language Server", serverOptions, clientOptions);
  context.subscriptions.push(
    client.start(),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration("goop.path")) {
        vscode.commands.executeCommand("workbench.action.reloadWindow");
      }
    }),
  );
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
