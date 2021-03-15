---
name: Bug report
about: Create a report to help us improve slo-exporter
title: "[BUG]"
labels: bug
assignees: ''

---
<details>
<summary>Please read this before submitting a bug report</summary>

* **Check the [debugging guide](docs/operating.md).** You might be able to find the cause of the problem and fix things yourself. Most importantly, check if you can reproduce the problem in the latest version of slo-exporter.
* **Perform a [cursory search](https://github.com/search?q=+is%3Aissue+repo%3Aslo-exporter)** to see if the problem has already been reported. If it has **and the issue is still open**, add a comment to the existing issue instead of opening a new one.

#### How Do I Submit A (Good) Bug Report?

Explain the problem and include additional details to help maintainers reproduce the problem:

* **Use a clear and descriptive title** for the issue to identify the problem.
* **Describe the exact steps which reproduce the problem** in as many details as possible. For example, start by explaining how you started slo-exporter, e.g. which command exactly you used in the terminal. When listing steps, **don't just say what you did, but explain how you did it**.
* **Provide specific examples to demonstrate the steps**. Include links to files or GitHub projects, or copy/pasteable snippets, which you use in those examples. If you're providing snippets in the issue, use [Markdown code blocks](https://help.github.com/articles/markdown-basics/#multiple-lines).
* **Describe the behavior you observed after following the steps** and point out what exactly is the problem with that behavior.
* **Explain which behavior you expected to see instead and why.**
* **If you're reporting that slo-exporter crashed**, include a crash report with a stack trace from the operating system. Include the crash report in the issue in a [code block](https://help.github.com/articles/markdown-basics/#multiple-lines), a [file attachment](https://help.github.com/articles/file-attachments-on-issues-and-pull-requests/), or put it in a [gist](https://gist.github.com/) and provide link to that gist.
* **If the problem is related to performance or memory**, include a [CPU profile capture](docs/operating.md#profiling) with your report.
* **If the problem wasn't triggered by a specific action**, describe what you were doing before the problem happened and share more information using the guidelines below.

Provide more context by answering these questions:

* **Did the problem start happening recently** (e.g. after updating to a new version) or was this always a problem?
* If the problem started happening recently, **can you reproduce the problem in an older version of slo-exporter?** What's the most recent version in which the problem doesn't happen?
* **Can you reliably reproduce the issue?** If not, provide details about how often the problem happens and under which conditions it normally happens.

Include details about your configuration and environment:

* **Which version are you using?** You can get the exact version by running `slo-exporter --version` in your terminal.
* **What's the name and version of the OS you're using**?
* **Are you running slo-exporter in a virtual machine or container?** If so, which VM software are you using and which operating systems and versions are used for the host and the guest?
* **What are your [local configuration files](docs/configuration.md) and environment variables?** `slo_exporter.yaml` and possibly others.
</details>
---

#### Describe the bug
FILL ME

#### How to reproduce the bug
FILL ME

#### Expected behavior
A clear and concise description of what you expected to happen.

#### Additional context
FILL ME
