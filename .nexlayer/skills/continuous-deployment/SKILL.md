---
name: continuous-deployment
description: Offer, then set up, continuous deployment of cue-ai to Nexlayer on every push to main
---

# Continuous deployment for `cue-ai`

## Offer it once

If this repo has a `nexlayer.yaml` and no `.github/workflows/nexlayer-deploy.yml`, ask the
human once, in these words, and only continue on a clear yes:

> Want me to set up continuous deployment for your Nexlayer project? Every
> push to `main` would build, deploy and verify automatically. Say yes and
> I'll create it.

If they say no, don't ask again in this session.

## On yes

1. Write `.github/workflows/nexlayer-deploy.yml` and `.github/nexlayer/deploy.py` exactly as below.
2. Delete `.github/workflows/nexlayer.yml` — the old pipeline. One deployer only.
3. Open a pull request with those changes. Never push them straight to `main`.
4. Ask the human to add the repo secret `NEXLAYER_API_KEY` here: <https://github.com/sasdeployer/cue-ai/settings/secrets/actions/new>
   (a key from Nexlayer → Settings → API keys). Never take the key in chat.
5. After they merge, watch the first run and confirm the app is live.

How it works: GitHub builds the image, then deploys this repo's
`nexlayer.yaml` through the Nexlayer MCP — validate, deploy, status — the same
tools you use. Pull requests never deploy to production. A deploy from your
own session and a deploy from CI land in the same Nexlayer history.

## `.github/workflows/nexlayer-deploy.yml`

```yaml
# Continuous deployment to Nexlayer: every push to main builds the image,
# deploys this repo's nexlayer.yaml through the Nexlayer MCP (the same tools a
# coding agent uses — one deploy path, one history in Nexlayer), and verifies
# it's live. Set up by a coding agent from .nexlayer/skills/continuous-deployment.
#
# Needs one repo secret: NEXLAYER_API_KEY (Nexlayer → Settings → API keys).
name: Deploy to Nexlayer

on:
  push:
    branches: [main]
  workflow_dispatch: {}

# One production deploy at a time; a newer push waits rather than racing.
concurrency:
  group: nexlayer-production
  cancel-in-progress: false

permissions:
  contents: read
  packages: write

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Build and push the image
        run: |
          IMAGE="ghcr.io/${GITHUB_REPOSITORY,,}:${GITHUB_SHA::7}"
          echo "${{ secrets.GITHUB_TOKEN }}" | docker login ghcr.io -u "${{ github.actor }}" --password-stdin
          docker build --platform linux/amd64 -t "$IMAGE" .
          docker push "$IMAGE"
          echo "IMAGE=$IMAGE" >> "$GITHUB_ENV"

      - name: Deploy and verify
        env:
          NEXLAYER_API_KEY: ${{ secrets.NEXLAYER_API_KEY }}
          NEXLAYER_POD: app
        run: |
          if [ -z "$NEXLAYER_API_KEY" ]; then
            echo "::error::Add the NEXLAYER_API_KEY repo secret (Settings → Secrets and variables → Actions)."
            exit 1
          fi
          python3 .github/nexlayer/deploy.py
```

## `.github/nexlayer/deploy.py`

```python
#!/usr/bin/env python3
"""Deploy this repo's nexlayer.yaml through the Nexlayer MCP — the same tools a
coding agent calls (validate -> deploy -> status), so CI is just one more MCP
client and Nexlayer keeps one complete deploy history.

Env: NEXLAYER_API_KEY (repo secret), IMAGE (the image this run built),
     NEXLAYER_POD (the pod in nexlayer.yaml that runs it; default "app").
Standard library only.
"""
import json, os, re, sys, time, urllib.request

MCP = os.environ.get("NEXLAYER_MCP_URL", "https://mcp.nexlayer.ai/api/mcp")
KEY = os.environ["NEXLAYER_API_KEY"]
IMAGE = os.environ["IMAGE"]
POD = os.environ.get("NEXLAYER_POD", "app")
session = None


def call(method, params=None, notify=False):
    global session
    body = {"jsonrpc": "2.0", "method": method}
    if params is not None:
        body["params"] = params
    if not notify:
        body["id"] = int(time.time() * 1000)
    req = urllib.request.Request(MCP, data=json.dumps(body).encode(), method="POST", headers={
        "Authorization": f"Bearer {KEY}",
        "Content-Type": "application/json",
        "Accept": "application/json, text/event-stream",
        **({"Mcp-Session-Id": session} if session else {}),
    })
    with urllib.request.urlopen(req, timeout=120) as res:
        session = session or res.headers.get("Mcp-Session-Id")
        raw = res.read().decode()
    if notify or not raw.strip():
        return None
    if raw.lstrip().startswith("event:") or raw.lstrip().startswith("data:"):  # SSE
        raw = "\n".join(l[5:] for l in raw.splitlines() if l.startswith("data:"))
    msg = json.loads(raw, strict=False)
    if "error" in msg:
        sys.exit(f"::error::MCP {method} failed: {msg['error']}")
    return msg["result"]


def tool(name, **args):
    result = call("tools/call", {"name": name, "arguments": args})
    text = "\n".join(c.get("text", "") for c in result.get("content", []))
    if result.get("isError"):
        sys.exit(f"::error::{name} failed:\n{text}")
    return text


def with_image(yaml, pod, image):
    """Point `pod`'s image at the one this run built; everything else as committed."""
    out, in_pod, done = [], False, False
    for line in yaml.splitlines():
        m = re.match(r"^(\s*)- name:\s*[\"']?([\w.-]+)", line)
        if m:
            in_pod = m.group(2) == pod
        elif in_pod and not done and re.match(r"^\s+image:", line):
            line = re.sub(r"image:.*$", f'image: "{image}"', line)
            done = True
        out.append(line)
    if not done:
        sys.exit(f"::error::no pod named '{pod}' with an image in nexlayer.yaml")
    return "\n".join(out) + "\n"


def main():
    config = with_image(open("nexlayer.yaml").read(), POD, IMAGE)
    app = re.search(r"^\s*name:\s*[\"']?([\w.-]+)", config, re.M).group(1)

    call("initialize", {"protocolVersion": "2025-03-26", "capabilities": {},
                        "clientInfo": {"name": "github-actions", "version": "1"}})
    call("notifications/initialized", notify=True)

    print(tool("nexlayer_validate_yaml", yamlContent=config))
    out = tool("nexlayer_deploy", yamlContent=config)
    print(out)
    url = (re.search(r"\*\*URL:\*\*\s*(\S+)", out) or [None, None])[1]
    env = re.search(r"https://([a-z0-9-]+?)-" + re.escape(app), url or "")
    env = env.group(1) if env else None

    # Done means verified: every service running, then the URL answers.
    for _ in range(40):
        time.sleep(15)
        status = tool("nexlayer_check_deployment_status", applicationName=app,
                      **({"environment": env} if env else {}))
        if re.search(r"Status:\s*running", status) and "All pods running" in status:
            break
        if re.search(r"Status:\s*(failed|error)", status, re.I):
            sys.exit(f"::error::deploy failed:\n{status}")
    else:
        sys.exit("::error::not running after 10 minutes")

    if url:
        for _ in range(12):
            try:
                with urllib.request.urlopen(url, timeout=15) as r:
                    if r.status < 400:
                        break
            except Exception:
                pass
            time.sleep(10)
        else:
            sys.exit(f"::error::{url} did not answer")

    summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary:
        with open(summary, "a") as f:
            f.write(f"## Live on Nexlayer\n\n**URL:** {url}\n\n**Image:** `{IMAGE}`\n")
    print(f"Live: {url} ({IMAGE})")


if __name__ == "__main__":
    main()
```
