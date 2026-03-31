# Troubleshooting

## Claude Code can't authenticate

**Symptom**: The agent starts but fails with an authentication error.

**Cause**: The `.claude` directory and `.claude.json` file from your host are not properly mounted into the container.

**Fix**:
1. Make sure you've run `claude login` on your host machine first
2. Check that `~/.claude` and `~/.claude.json` exist on your host
3. Verify the volume mounts in `docker-compose.yml`:
   ```yaml
   volumes:
     - ${HOME}/.claude:/home/agent/.claude
     - ${HOME}/.claude.json:/home/agent/.claude.json
   ```

## Git push/pull fails

**Symptom**: The agent can commit locally but fails when pushing to or pulling from the remote.

**Cause**: Git credentials are not available inside the container.

**Fix (SSH)**:
Add your SSH directory as a read-only mount in `docker-compose.yml`:
```yaml
volumes:
  - ${HOME}/.ssh:/home/agent/.ssh:ro
```

**Fix (HTTPS)**:
Set a personal access token via environment variable:
```yaml
environment:
  - GITHUB_TOKEN=${GITHUB_TOKEN}
```

Or configure git credential storage in the Dockerfile.

## Container won't start — permission denied

**Symptom**: `docker compose up` fails with permission errors on mounted volumes.

**Cause**: The container user's UID/GID doesn't match your host user.

**Fix**: Set `UID` and `GID` when building:
```
UID=$(id -u) GID=$(id -g) docker compose build
```

Or set them in a `.env` file:
```
UID=1000
GID=1000
```

## Agent skips tests

**Symptom**: The agent implements features but doesn't write or run tests.

**Cause**: `testing.enabled` is set to `false` in `agent.yaml`, or the `testing.command` is empty.

**Fix**: Check your `agent.yaml`:
```yaml
testing:
  enabled: true
  command: "pytest -v"  # Must be a valid command that exits 0 on success
```

## Worktree creation fails

**Symptom**: `git worktree add` fails during Step 3.

**Possible causes**:
1. The `worktrees/` directory doesn't exist — create it: `mkdir -p worktrees`
2. A worktree from a previous crashed session still exists — clean it up:
   ```
   git worktree prune
   ```
3. The feature branch already exists from a previous attempt:
   ```
   git branch -D feature/<name>
   ```

## Agent runs out of context mid-feature

**Symptom**: The agent stops responding or produces degraded output partway through a feature.

**This is expected behavior.** The agent is designed to survive this:
1. Each plan step produces an independent commit
2. The plan file records what has been completed
3. Re-run `scripts/agent.sh` and the agent will resume from where it left off

To reduce the frequency:
- Keep plan steps small (each should be implementable in ~100-200 lines of code)
- Increase the model's context window if a larger model is available
- Break large features into smaller backlog items

## Container uses too much memory

**Symptom**: The container is killed by OOM or the host becomes unresponsive.

**Fix**: Adjust the memory limit in `docker-compose.yml` or via environment variable:
```
AGENT_MEMORY=4G scripts/agent.sh
```

## Tests pass locally but fail in the container

**Symptom**: Your test suite works on the host but fails inside the container.

**Common causes**:
1. **Missing system libraries**: The container has a minimal Ubuntu install. Add missing packages to the Dockerfile.
2. **Different tool versions**: The container might have a different version of your language/framework. Pin versions in the Dockerfile.
3. **No display server**: If tests need a GUI, make sure `DISPLAY=:99` is set and `xvfb` is in the Dockerfile (it is in the base image).
4. **Path differences**: The working directory inside the container is `/workspace`, not your host path. Use relative paths in config files.
