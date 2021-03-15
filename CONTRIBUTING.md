#### Table Of Contents

[Code of Conduct](#code-of-conduct)

[I just have a question](#i-just-have-a-question)

[Your First Code Contribution](#your-first-code-contribution)

[Pull Requests](#pull-requests)

[Styleguides](#styleguides)

### Code of Conduct
This project and everyone participating in it is governed by the [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.


## I just have a question
Please file an issue with the `question` label or contact us via [Slack](/README.md#community).

### Your First Code Contribution

Unsure where to begin contributing to slo-exporter? You can start by looking through these `good-first-issue` and `help-wanted` issues:

* [Good first issues](https://github.com/seznam/slo-exporter/labels/good%20first%20issue) - issues which should only require a few lines of code, and a test or two.
* [Help wanted issues](https://github.com/seznam/slo-exporter/labels/help%20wanted) - issues which should be a bit more involved than `good-first-issues`.

### Pull Requests

Please follow these steps to have your contribution considered by the maintainers:

2. Follow the [styleguides](#styleguides)
3. After you submit your pull request, verify that all [status checks](https://help.github.com/articles/about-status-checks/) are passing <details><summary>What if the status checks are failing?</summary>If a status check is failing, and you believe that the failure is unrelated to your change, please leave a comment on the pull request explaining why you believe the failure is unrelated.</details>

While the prerequisites above must be satisfied prior to having your pull request reviewed, the reviewer(s) may ask you to complete additional design work, tests, or other changes before your pull request can be ultimately accepted.

## Styleguides

### Git Commit Messages

* Use the present tense ("Add feature" not "Added feature")
* Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
* Limit the first line to 72 characters or less
* Reference issues and pull requests liberally after the first line

### Golang Styleguide

Follow golang [revive](github.com/mgechev/revive) advices and make sure revive reports same or less issues.

### Documentation Styleguide

* Use [Markdown](https://daringfireball.net/projects/markdown).
