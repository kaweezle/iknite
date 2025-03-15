# Contributing to Octave Cloud Platform

!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

#### Table Of Contents

[Code of conduct](#code-of-conduct)

- [General considerations](#general-considerations)
- [Normal contribution flow](#general-considerations)
- [Creating issues](#creating-issues)
- [Commit guidelines](#commit-guidelines)
- [Code style](#code-style)

## Code of Conduct

This project and everyone participating in it is governed by the
[Octave Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected
to uphold this code. Please report unacceptable behavior to
[antoine@openance.com](mailto:antoine@openance.com).

## General considerations

- `main` is the development branch.
- Branches prefixed with `deploy/` are continuous deployment branches created
  for CI/CD builds. They may be created by scripts.
- Tags with the form `component_vX.Y.Z` trigger builds in Github Actions.
- We use [Semantic Versioning](https://semver.org/)
- Use only lower case in branch names. Prefer minus (`-`) instead of underscore
  (`_`) for symbolizing spaces as github replaces them with spaces in PR titles.
- Please prefix your branch names. Try not to be too creative with prefixes and
  choose one of the following:
  - `feature/`
  - `fix/`
  - `build/`
  - `doc/`
- We use [gitmoji](https://gitmoji.dev/) in particular with the
  [Visual Studio Code Plugin](https://marketplace.visualstudio.com/items?itemName=seatonjiang.gitmoji-vscode)

## Internal contribution flow

The contribution flow is the following:

1. Creating an issue to track the contribution.
2. Create a branch from **main** and add the `OnGoing` label to the issue.
3. Do some work in the branch.
4. Rebase your branch before pushing.
5. Push your branch. If needed, it should trigger some CI building and testing.
6. Create a pull request. If your pull request must not be included in staging
   builds, label it as `WIP` (Work In Progress).
7. Assign the PR to one of your colleagues for review. Don't ask for a review on
   a PR that doesn't build or for which test fails. Remove the `OnGoing` flag
   from the issue and add the `To Review` Flag.
8. Wait for approval from one of your peers and from QA or the PO (on the
   issue). Answer to each comment and mark them as resolved if needed. The
   approval from the peer goes in the PR and the one from QA is given by adding
   the `OkQA` flag.
9. Once the PR is approved, you can merge it into main. Don't forget to delete
   the branch. The issue will be closed automatically.

You have more information in
[this blog post](https://about.gitlab.com/2016f/03/08/gitlab-tutorial-its-all-connected/).

## Creating issues

Each contribution must be related to an issue in
[Github](https://github.com/kaweezle/iknite/issues). Each issue should be of the
appropriate type: Task, Epic, Bug...

### Story

This [gist](https://gist.github.com/xpepper/4ed5638d5a431b32f573) shows the
basic idea to follow on the story issue title:

    As a <user or stakeholder type>
    I want <some software feature>
    So that <some business value>

The content of the story should at least detail the acceptance criteria. For the
contents, you can follow
[this guide](https://github.com/AlphaFounders/style-guide/blob/master/agile-user-story.md)

### Epic

An Epic is a subject matter of the project, that will be relevant for several
Stories and will cross multiple Milestones. A good example is for instance
_Authentication_. There will be several Stories related to it. Creating an Epic
titled `Epic: Security` allows to present and discuss the principles related to
the feature And link the varkious stories and bugs related to it.

For most if not all of them, it is interesting to also create a Label and a
Documentation page. The former is an easier way to group issues related to the
Epic and the latter allows to document the feature globally. The interest of
having an issue related to the epic is the ability to track down work related to
it and have a conversation about it.

### Bug

As the term _Issue_ covers also the stories and epics, we are left with _Bug_
for describing real issues in the project. They can be real bugs, but also
crashes and anoyments.

I don't like to be too specific on the contents of a Bug, but please try to
follow at least these guidelines:

- Describe how to reproduce.
- Describe what happens.
- Describe **what is expected** and how this is different from what happens.
- Describe the context as thorougly as possible. In particular, give information
  about: OS, device model, user context

## Commit guidelines

Before reading this, you need to know how to
[rebase](https://www.atlassian.com/git/tutorials/rewriting-history/git-rebase)
and
[rewrite history](https://git-scm.com/book/en/v2/Git-Tools-Rewriting-History).

Please follow these rules in your commits:

- Keep your commit as minimal as possible.
- When a commit fixes an issue introduced in preceding commit, please squash it
  with the preceding one.
- keep formatting changes and/or renames in their own commits. Don't reformat
  files if they don't need to.
- When you fix a test in a latter commit, please squash it with the preceding
  one.
- Remove empty commits occuring from rebase.

### Commit messages

Please follow
[this guide](https://gist.github.com/robertpainsi/b632364184e70900af4ab688decf6f53).

For those who like it, you can follow the
[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/)
guidelines and use the [gitmoji](https://gitmoji.dev/) convention for the commit
type.

You can add link in messages:

- To reference an issue: `#123`
- To reference a PR: `!123`
- To reference a snippet `$123`

## Code style

The general guideline here is: **FOLLOW THE CODE STYLE OF THE PLATFORM**. In the
case of the current project, good references are
[Effective Go](https://go.dev/doc/effective_go) and
[The Go Style Guide](https://google.github.io/styleguide/go/).

Here is some dos and donts:

- Use something to separate words: case, underscore... Use what you see in the
  constructs of the language and use this pattern **consistently**.
- Don't abbreviate. Really, don't.
- Fix typos as soon as you see them.
- Use only plain english in names.
- Use verbs for methods and nouns for properties.
- Keep you methods small.
- Use loose coupling where you can.
- When you fail, fail fast and vocally. Never fail silently.

## Documentation

You are greatly encouraged to contribute documentation to the project. The main
project documentation is located in the `docs` directory. It is written in
[Markdown](https://daringfireball.net/projects/markdown/syntax) and is produced
with the [MkDocs](https://www.mkdocs.org/) tool with the help of
[Material for MKDocs](https://squidfunk.github.io/mkdocs-material/) and
[PyMdown Extensions](https://facelessuser.github.io/pymdown-extensions/). You
may write documentation in other parts of the project but please don't forget to
create a link for the created file somewhere in the `docs` directory tree and
make sure that one of the existing pages link to it or that it is linked in the
`mkdocs.yml` file.

For other that pull your changes from git, the link you create needs to be
relative:

```bash
$ cd docs
$ mkdir -p some/new/documentation/category
$ cd some/new/documentation/category
$ ln -s ../../../../my/project/subdir/README.md .
```

You can add categories inside the `/mkdocs.yml` file.

You can see and browse the documentation while you're editing it by installing
[MkDocs](https://www.mkdocs.org/) locally and serving the documentation (In the
following we assme that you are either on Linux or MacOS with python3
installed):

```console
$ python3 -mvenv .venv
$ source .venv/bin/activate
(env) $ pip intall -r mkdocs-requirements.txt
(env) $ mkdocs serve
INFO    -  Building documentation...
INFO    -  Cleaning site directory
[I 190315 09:36:39 server:298] Serving on http://127.0.0.1:8000
[I 190315 09:36:39 handlers:59] Start watching changes
[I 190315 09:36:39 handlers:61] Start detecting changes
...
```

Then you can browse the documentation at
[http://localhost:8000/](http://localhost:8000/)
