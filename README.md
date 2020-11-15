<p align="center"><br><br><br><br>
:steam_locomotive:<br>
<b>GitHub Issues Mover</b>
</p>

<p align="center">
This is a CLI tool to migrate issues across GitHub and GitHub Enteprise repos.<br>
In addition to issues, migration also includes labels and milestones.
</p><br><br><br><br>

Installation
--

To install, use `go get`:

```sh
$ go get github.com/linyows/github-issues-mover
```

Usage
--

Example:

```sh
$ export SRC_TOKEN=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
$ export DST_TOKEN=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
$ github-issues-mover -src=foo/bar -dst=foo/bar -dst-endpoint=https://ghe.yourhost.com
```

Contribution
------------

1. Fork ([https://github.com/linyows/github-issues-mover/fork](https://github.com/linyows/github-issues-mover/fork))
1. Create a feature branch
1. Commit your changes
1. Rebase your local changes against the master branch
1. Run test suite with the `go test ./...` command and confirm that it passes
1. Run `gofmt -s`
1. Create a new Pull Request

Author
------

[linyows](https://github.com/linyows)
