use zed_extension_api as zed;

struct GoopExtension;

impl zed::Extension for GoopExtension {
    fn new() -> Self {
        Self
    }

    fn language_server_command(
        &mut self,
        _language_server_id: &zed::LanguageServerId,
        worktree: &zed::Worktree,
    ) -> zed::Result<zed::Command> {
        let path = worktree
            .which("goop")
            .ok_or_else(|| {
                "Goop language server not found. Build it: `cd src && go build ./cmd/goop` and ensure `goop` is in your PATH.".to_string()
            })?;

        Ok(zed::Command {
            command: path,
            args: vec!["lsp".to_string()],
            env: Default::default(),
        })
    }
}

zed::register_extension!(GoopExtension);
