<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# CLI mode


You can invoke any tool directly from the command line without running an MCP server. This is useful for shell scripts, CI/CD pipelines, and Claude Code skills.

```bash
# List all available tools (grouped by domain)
forgejo-mcp --cli list

# Invoke a tool with JSON arguments
forgejo-mcp --cli get_issue_by_index --args '{"owner":"OWNER","repo":"REPO","index":1}'

# Pipe JSON arguments via stdin
echo '{"owner":"OWNER","repo":"REPO"}' | forgejo-mcp --cli list_repo_issues

# List recent workflow runs (text output)
forgejo-mcp --cli list_workflow_runs \
  --args '{"owner":"OWNER","repo":"REPO"}' \
  --output=text

# List only failed runs
forgejo-mcp --cli list_workflow_runs \
  --args '{"owner":"OWNER","repo":"REPO","status":"failure"}' \
  --output=text

# List jobs and inspect the tail of a failed job (Forgejo v16+)
forgejo-mcp --cli list_action_run_jobs \
  --args '{"owner":"OWNER","repo":"REPO","run_id":123}' \
  --output=text
forgejo-mcp --cli get_action_job_logs \
  --args '{"owner":"OWNER","repo":"REPO","job_id":456,"max_bytes":32768}' \
  --output=text

# Forgejo's run-wide ZIP log endpoint has no Range support. Enumerate jobs and
# fetch their bounded plaintext logs instead.

# List artifacts of a run, then read one artifact's metadata (no zip download)
forgejo-mcp --cli list_action_run_artifacts \
  --args '{"owner":"OWNER","repo":"REPO","run_id":123,"limit":30}' \
  --output=text
forgejo-mcp --cli get_action_artifact \
  --args '{"owner":"OWNER","repo":"REPO","artifact_id":789}' \
  --output=text

# List package versions for an owner, then inspect one version's files
forgejo-mcp --cli list_packages \
  --args '{"owner":"OWNER","type":"container","limit":30}' \
  --output=text
forgejo-mcp --cli get_package \
  --args '{"owner":"OWNER","type":"container","name":"app","version":"1.0.0"}' \
  --output=text
forgejo-mcp --cli list_package_files \
  --args '{"owner":"OWNER","type":"container","name":"app","version":"1.0.0"}' \
  --output=text

# delete_package removes one version. Do not invoke it against a registry you do not own.

# cancel_workflow_run is 204 even when the run already finished.
# delete_workflow_run only succeeds for a completed run; a live run is an error.

# Show a tool's parameters
forgejo-mcp --cli create_issue --help

# Control output format (json or text)
forgejo-mcp --cli list --output=json
forgejo-mcp --cli get_my_user_info --args '{}' --output=text
```

CLI mode requires the same `FORGEJO_URL` and `FORGEJO_ACCESS_TOKEN` configuration as MCP server mode. Tool results are written as JSON to stdout by default; errors go to stderr with a non-zero exit code.

