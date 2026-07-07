use zed_extension_api as zed;

struct C0Extension;

impl zed::Extension for C0Extension {
    fn new() -> Self {
        Self
    }

    fn language_server_command(
        &mut self,
        _language_server_id: &zed::LanguageServerId,
        worktree: &zed::Worktree,
    ) -> zed::Result<zed::Command> {
        let path = worktree
            .which("c0")
            .ok_or_else(|| {
                "C0 language server not found. Build it: `cd src && go build ./cmd/c0` and ensure `c0` is in your PATH.".to_string()
            })?;

        Ok(zed::Command {
            command: path,
            args: vec!["lsp".to_string()],
            env: Default::default(),
        })
    }
}

zed::register_extension!(C0Extension);
