# Third-party notices

nameary is distributed under the Apache License 2.0. It depends on the
following open-source projects. Each project remains under its own license and
copyright; this file is an index, not a replacement for the license text
shipped in each module.

| Module | License |
|---|---|
| charm.land/bubbletea/v2 | MIT |
| charm.land/bubbles/v2 | MIT |
| charm.land/lipgloss/v2 | MIT |
| charm.land/huh/v2 | MIT |
| charm.land/log/v2 | MIT |
| github.com/charmbracelet/harmonica | MIT |
| github.com/charmbracelet/x/ansi | MIT |
| github.com/charmbracelet/x/term | MIT |
| github.com/spf13/cobra | Apache-2.0 |
| go.etcd.io/bbolt | MIT |
| github.com/BurntSushi/toml | MIT |
| golang.org/x/net | BSD-3-Clause |
| golang.org/x/sync | BSD-3-Clause |

Transitive dependencies retain the licenses declared by their modules. Run
`go mod download -json` and inspect each module's `LICENSE` file for the full
texts. If a future dependency uses a copyleft or notice-heavy license, add its
notice here before release.
