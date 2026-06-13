---
title: "Quick start"
description: "Run your first cacm command."
weight: 30
---

Once `cacm` is on your `PATH`:

```bash
cacm --help       # see the command tree
cacm version      # build info
```

This is a fresh scaffold, so the command tree is just `version` for now. Add
your first real command in `cli/`, build on the `cacm` library package,
and document it here.

A good first command usually fetches one thing and prints it as JSON, so the
output pipes straight into `jq` and the rest of your tools.
