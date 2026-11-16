<div align="center">

[English](README.md) | 简体中文

<h1>nameary</h1>

<p><strong>起名之前，先查一遍。</strong></p>

<p>
  <a href="https://github.com/yufeiminds/nameary/actions/workflows/ci.yml"><img src="https://github.com/yufeiminds/nameary/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://pkg.go.dev/github.com/yufeiminds/nameary"><img src="https://pkg.go.dev/badge/github.com/yufeiminds/nameary.svg" alt="Go Reference"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/yufeiminds/nameary" alt="Go Version"></a>
  <a href="https://github.com/yufeiminds/nameary/releases"><img src="https://img.shields.io/github/v/release/yufeiminds/nameary?include_prereleases&sort=semver" alt="Latest Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/yufeiminds/nameary" alt="License"></a>
  <img src="https://img.shields.io/badge/platforms-macOS%20%7C%20Linux%20%7C%20Windows-2ea44f" alt="Platforms">
  <a href="#安装"><img src="https://img.shields.io/badge/Homebrew-install-FBB040?logo=homebrew&logoColor=white" alt="Homebrew"></a>
  <a href="#安装"><img src="https://img.shields.io/badge/mise-install-6E56CF" alt="mise"></a>
</p>

</div>

`nameary` 会并行检查一个名字在 **20 个默认来源**——域名、软件包仓库、平台句柄——上的可用性，并随检查完成即时输出结果。在终端中默认打开全屏交互式 TUI；通过管道使用时则输出纯文本表格。

```bash
nameary acme
```

![nameary 演示](demo/nameary.gif)

| TUI | Console |
|---|---|
| ![TUI](demo/tui.png) | ![Console](demo/console.png) |

## 为什么它感觉这么快

- **即时反馈。** TUI 在第一个网络请求完成之前就把每一行画成骨架；`--format console` 先打印 logo 和分组，随后每个检查完成即输出一行结果。
- **第一条真实结果通常 < 400ms。** 快的来源与较慢的域名检查同时启动。
- **不会突然重排。** TUI 保持行序稳定；落定的行只闪烁一次，不会跳来跳去。
- **取消是真取消。** `Ctrl+C`（TUI 中也可按 `q`）会取消在途请求。
- **诚实的输出。** 超时、429 与 404 是不同的状态。只有明确的“未找到”才会成为 `available`。

## 安装

### Homebrew（推荐）

```bash
brew install yufeiminds/tap/nameary
```

### mise（推荐）

```bash
mise use -g github:yufeiminds/nameary@latest
```

两种方式安装的都是预编译二进制，无需 Go 工具链。

### 其他方式

从 [GitHub Releases](https://github.com/yufeiminds/nameary/releases) 下载对应平台的压缩包，或使用 **Go 1.26+** 从源码构建：

```bash
go install github.com/yufeiminds/nameary/cmd/nameary@latest
```

或从检出目录构建：

```bash
git clone https://github.com/yufeiminds/nameary
cd nameary
go build -o nameary ./cmd/nameary
```

## 第一次运行

```bash
nameary acme
```

每一项计划中的检查都会先以骨架行的形式立即出现，并随各来源返回而逐行填充——首批结果通常在几百毫秒内到达。按 `?` 查看帮助，`q` 退出。

### 读懂结果

每一行对应一个来源，带有状态和置信度：

| 状态 | 含义 |
|---|---|
| `available` | 来源明确返回“不存在”。 |
| `taken` | 该名字在此来源已被占用。 |
| `timeout` | 来源在超时时间内没有响应。 |
| `rate_limit` | 来源返回 429 或配额耗尽。 |
| `conflict` | GitHub 上存在同名高星项目；这是提示信号，不代表名称不可用。 |
| `unknown` | 无法判定：WHOIS 被阻断、结果截断或响应无法分类。绝不会报告为 `available`。 |
| `error` / `skipped` / `canceled` | 无结论：来源出错、检查无法执行（例如名字在该来源不合法），或运行被中断。 |

置信度衡量 `available` 结果的可信程度：`certain`——来源自身 API 给出的答案；`likely`——较强的信号（例如 RDAP 中没有该域名的注册记录）；`low`——较低。

> `available` **并不**是保证。保留域名和溢价域名仍可能不可注册或价格更高——依赖结果前请先向域名注册商确认。可用性也不等同于商标清查（见[范围与注意事项](#范围与注意事项)）。

## 实用场景

### 名字被占用时让 CI 失败

```bash
nameary check acme --format json --fail-if-taken
```

任何被检查来源报告 `taken` 时，`--fail-if-taken` 会以退出码 3 结束，可直接嵌入脚本或 workflow 步骤。全部退出码见[参考](#退出码)。

### 检查完整域名

能解析为域名的名字只会检查该域名：

```bash
nameary acme.dev                 # 只检查域名 acme.dev
nameary acme.dev --targets npm   # ...或显式追加包管理器目标
```

显式指定 `--targets` 或 `--preset` 时，始终优先于域名捷径。

### 按技术栈检查

预设为某个技术栈打包了一组目标和 TLD——完整表格见[预设参考](#预设)：

```bash
nameary myname --preset go       # 域名、GitHub、Homebrew + .com/.io/.dev
nameary myname --preset rust
```

### 自选检查来源

```bash
nameary acme --targets npm,pypi,github
nameary acme --targets all --tlds com,dev
```

### 避免 GitHub 限流

没有 token 时，GitHub 允许每小时 60 次请求，`nameary` 会退回公共主页检查并使用 12 小时缓存。设置 token 可使用 REST API 并获得更丰富的详情：

```bash
export GITHUB_TOKEN=ghp_…        # GH_TOKEN 也可以
nameary acme
```

token 也可以通过 `--github-token` 或配置文件的 `[github]` 段提供。

### 获取机器可读输出

管道时默认 `table`；`json` 在全部检查完成后输出一个数组；`ndjson` 每个检查落地即输出一个对象：

```bash
nameary check acme --format ndjson | jq .
```

各格式详见[输出格式](#输出格式)。

### 控制缓存

结果按目标缓存。可以完全重新检查，或找到并清空缓存数据库：

```bash
nameary acme --cache-mode off      # 跳过缓存
nameary acme --cache-mode refresh  # 忽略缓存结果，重新检查
nameary cache path
nameary cache clear
```

### 纯净与无障碍输出

```bash
nameary acme --style plain
```

`plain` 是 ASCII 字形 + 无颜色 + 减少动画的简写；`--style ascii,calm` 可组合单项。`NO_COLOR`、`CLICOLOR=0`、`NAMEARY_REDUCED_MOTION=1` 和 `TERM=dumb` 会被自动识别（见[环境变量](#环境变量)）。

## 参考

### 命令

| 命令 | 用途 |
|---|---|
| `nameary [name]` | 检查一个名字（TTY 下默认交互式 TUI） |
| `nameary check <name>` | 精确检查一个名字；旗标与输出格式相同 |
| `nameary version` / `nameary --version` | 打印版本信息 |
| `nameary cache path` | 打印缓存数据库路径 |
| `nameary cache clear` | 删除缓存数据库 |
| `nameary config` / `nameary config path` | 打印配置文件路径 |
| `nameary config init` | 创建配置文件（TTY 下交互式；非交互可用 `--force`、`--preset`、`--targets`、`--tlds`） |

### 旗标

| 旗标 | 说明 |
|---|---|
| `--preset` | `default`（默认）、`indie`、`web`、`go`、`rust`、`jvm`、`apple`、`linux`、`data`、`all` |
| `--targets` | 逗号分隔的目标列表，或 `all` |
| `--tlds` | 逗号分隔的 TLD 列表 |
| `--concurrency` | 全局并发检查数（默认 16） |
| `--timeout` | 单请求超时覆盖（默认 3s） |
| `--format` | `auto`、`tui`、`console`、`table`、`json`、`ndjson` |
| `--style` | `auto`、`plain`、`ascii`、`mono`、`calm`（逗号分隔） |
| `--only-available` | 只输出 available 结果 |
| `--cache-mode` | `auto`、`off`、`refresh` |
| `--cache-ttl` | 覆盖各目标的缓存 TTL |
| `--cache` | 缓存数据库路径 |
| `--celebrate` | 全部可用时显示祝贺提示 |
| `--fail-if-taken` | 有任何 taken 时以退出码 3 结束 |
| `--github-token` | GitHub token（或 `GITHUB_TOKEN` / `GH_TOKEN`） |
| `--github-min-stars` | `github-repos` 冲突检测的最低 star 数（默认 1000） |
| `--debug` | 将调试日志写入缓存目录 |

### 预设

| 预设 | 目标 | TLD |
|---|---|---|
| `default`（默认） | 20 个稳定来源 | com, io, dev, app, ai, co |
| `indie` | domains, npm, pypi, github | com, io, dev, app, ai, co |
| `web` | domains, npm, pypi, github, jsr | com, io, dev, app |
| `go` | domains, github, homebrew | com, io, dev |
| `rust` | domains, crates, github | com, io, dev |
| `jvm` | domains, maven, github | com, io, dev |
| `apple` | domains, cocoapods, github | com, io, dev |
| `linux` | domains, aur, snap, homebrew, github | com, io, dev |
| `data` | domains, conda, pypi, github | com, io, dev |
| `all` | 所有目标 | com, io, dev, app, ai, co |

### 检查目标

| 目标 | 来源 | “available” 的判定 |
|---|---|---|
| `domains` | DNS NS + RDAP（+ WHOIS 兜底） | 未找到注册记录 |
| `npm` | registry.npmjs.org | 404 |
| `pypi` | pypi.org/pypi | 404 |
| `crates` | crates.io API | 404 |
| `rubygems` | rubygems.org API | 404 |
| `nuget` | NuGet flat container | 404 |
| `dockerhub` | Docker Hub API | 无同名用户/组织命名空间，且无官方镜像 |
| `homebrew` | formulae.brew.sh | 无 formula 也无 cask |
| `packagist` | packagist.org | `vendor/package` 返回 404 |
| `hex` | hex.pm API | 404 |
| `pub` | pub.dev API | 404 |
| `github` | REST API 或公共主页 | 404 / 主页 404 |
| `github-repos` | GitHub Search API（opt-in，包含在 `all`） | 没有超过 star 阈值的精确同名仓库 |
| `maven` | Maven Central Solr 搜索 | 所有 group 中均无该 artifact ID |
| `conda` | Anaconda 包搜索 | 各 channel 均无同名包 |
| `cocoapods` | cocoapods.org/pods | 404 |
| `aur` | AUR RPC v5 | `resultcount` 为 0 |
| `snap` | Snap Store API | 404 |
| `chocolatey` | Chocolatey OData feed | Atom feed 为空 |
| `hackage` | hackage.haskell.org | 404 |
| `jsr` | JSR 搜索 / scoped meta | 无同名包 |

各来源的端点、限流与缓存 TTL：[`docs/sources.md`](docs/sources.md)。

### 退出码

| 退出码 | 含义 |
|---|---|
| `0` | 运行完成；没有 taken 结果（或未设置 `--fail-if-taken`） |
| `1` | 选项非法（未知 preset、style 或 cache mode）、配置问题，或运行失败 |
| `2` | 用法错误：名字多于一个、旗标放在名字之后、非 TTY 下缺少名字、无 TTY 却请求 TUI，或未知的 `--format` 值 |
| `3` | 设置了 `--fail-if-taken` 且至少有一个结果为 `taken` |

### 环境变量

| 变量 | 作用 |
|---|---|
| `GITHUB_TOKEN`、`GH_TOKEN` | 用于 GitHub REST API 的 token（也可通过 `[github] token_env` 配置） |
| `NAMEARY_CONFIG` | 配置文件路径 |
| `NO_COLOR` | 禁用颜色（通过管道时也会自动禁用） |
| `CLICOLOR=0` | 禁用颜色 |
| `NAMEARY_REDUCED_MOTION=1` | 减少动画 |
| `TERM=dumb` | 强制纯净、非交互行为 |

### 配置

配置文件位于用户配置目录——运行 `nameary config path` 可打印路径（Linux 上为 `~/.config/nameary/config.toml`，macOS 上为 `~/Library/Application Support/nameary/config.toml`）；`NAMEARY_CONFIG` 可覆盖该位置。

| 键 | 默认值 | 说明 |
|---|---|---|
| `preset` | `"default"` | 预设：决定 `targets` 和 `tlds` |
| `targets` | 来自预设 | 要检查的来源 |
| `tlds` | 来自预设 | 要检查的 TLD |
| `concurrency` | `16` | 全局并发检查数 |
| `timeout` | `"3s"` | 全局单请求超时覆盖 |
| `cache_ttl` | 各目标自定 | 全局缓存 TTL 覆盖，如 `"12h"` |
| `celebrate` | `false` | 全部可用时显示祝贺提示 |
| `[github] token_env` | `"GITHUB_TOKEN"` | 读取 GitHub token 的环境变量名 |
| `[github] min_stars` | `1000` | opt-in `github-repos` 冲突检测的最低 star 数 |

```toml
preset = "default"
tlds = ["com", "io", "dev", "app", "ai", "co"]
targets = ["domains", "npm", "pypi", "github"]
concurrency = 16
timeout = "3s"
# cache_ttl = "12h"   # 可选的全局覆盖；否则按各目标的 TTL 设置
celebrate = false

[github]
token_env = "GITHUB_TOKEN"
min_stars = 1000
```

优先级：旗标 > 环境变量 > 配置文件 > 默认值。

### 输出格式

| 格式 | 行为 |
|---|---|
| `auto` | 完整 TTY 下为 `tui`；stdout 是终端而 stdin 不是时为 `console`；管道时为 `table` |
| `tui` | 全屏交互式 TUI（需要 TTY） |
| `console` | 渐进式非交互输出：先打印 logo 和分组骨架，随后每个检查完成输出一行；终端过矮或使用 `--only-available` 时降级为分组块 |
| `table` | 纯文本对齐表格 |
| `json` | 全部检查完成后输出一个 JSON 数组 |
| `ndjson` | 每个检查落地即输出一个 JSON 对象 |

通过管道时自动禁用颜色。`ndjson` 的行长这样（一条 npm 检查和一条域名检查）：

```json
{"name":"acme","target":"npm","label":"acme","status":"taken","confidence":"certain","query":"acme","summary":"v3.0.3","url":"https://www.npmjs.com/package/acme","latency_ms":436}
{"name":"zzqxvfqk.dev","target":"domains","label":"zzqxvfqk.dev","status":"available","confidence":"likely","query":"zzqxvfqk.dev","summary":"no registration record","url":"https://pubapi.registry.google/rdap/domain/zzqxvfqk.dev","latency_ms":726}
```

### 快捷键（TUI）

| 按键 | 动作 |
|---|---|
| `↑`/`↓` 或 `k`/`j` | 移动选择 |
| `pgup` / `pgdown` | 翻页 |
| `g` / `G` | 跳到顶部 / 底部 |
| `enter` | 打开行详情 |
| `/` | 过滤行 |
| `a` | 切换只看 available |
| `s` | 循环排序：分组顺序 → 状态 → 延迟 |
| `tab` | 下一分组 |
| `r` | 重新运行检查 |
| `e` / `E` | 导出结果为 JSON / Markdown |
| `c` | 复制来源 URL |
| `o` | 在浏览器中打开来源 |
| `?` | 帮助 |
| `esc` | 返回 / 取消 |
| `q` 或 `ctrl+c` | 退出 |

## 结果是如何判定的

### 域名结果刻意保守

1. **DNS NS 查询**是快速的“已占用”信号。域名已委派即已注册，跳过 RDAP。
2. **RDAP** 是权威检查。带真实“对象不存在”响应体的 404 会成为 `available`，置信度 `likely`。
3. **无 RDAP 服务**（部分 ccTLD 常见）时退回 43 端口的 **WHOIS**。若 WHOIS 被阻断或无定论，结果为 `unknown`，绝不会是 `available`。
4. `available` **不是**保证：保留域名与溢价域名仍可能不可注册或价格更高。依赖结果前请先向注册商确认。

### 无 token 时的 GitHub

GitHub 匿名请求限额为每小时 60 次，因此没有 token 时 `nameary` 改查公共主页（`HEAD github.com/<name>`）——每秒 1 次请求、缓存 12 小时——单次运行不会耗尽配额。有 token 时使用 REST API 并返回更丰富的详情。

### GitHub 同名项目冲突

`github-repos` 是 opt-in 功能，因为 GitHub Search API 的限额更严格。它包含在 `--preset all` 和 `--targets all` 中，但不属于默认的 `default` preset：

```bash
nameary acme --targets github-repos
nameary acme --preset all
nameary acme --targets all,github-repos --github-min-stars 500
```

它搜索仓库名、筛选精确同名，并在达到 star 阈值时返回 `conflict`。TUI 会把冲突展开成项目子行，显示 stars、语言和描述；JSON/NDJSON 在 `projects` 字段中保留完整列表。它不影响 `--fail-if-taken`。建议设置 `GITHUB_TOKEN` 以获得可靠的搜索配额。

## 开发

```bash
go test ./...
go test -race ./internal/...
go test -count=1 ./e2e
go vet ./...
go build -o nameary ./cmd/nameary
```

测试分层、在线冒烟矩阵与本地 `act` CI 见 [`docs/testing.md`](docs/testing.md)。

从检出目录用 `go run` 运行时，注意包路径与 nameary 参数之间没有 `--`：

```bash
go run ./cmd/nameary acme
go run ./cmd/nameary acme --format console
```

演示图片/GIF 使用 **Git LFS** 存储。如需这些资源，请安装 `git-lfs` 并运行 `git lfs pull`；二进制与测试不依赖它们。重新录制演示（需要 [VHS](https://github.com/charmbracelet/vhs)）：

```bash
vhs demo/demo.tape
```

发布就绪检查：[`docs/release-checklist.md`](docs/release-checklist.md)。
发布说明与 schema：[`docs/release.md`](docs/release.md)。
各来源端点与限流：[`docs/sources.md`](docs/sources.md)。
安全策略：[`SECURITY.md`](SECURITY.md)；安全审计：[`docs/security-audit.md`](docs/security-audit.md)。

### 架构

```
Cobra ──► app ──┬─► TUI (default TTY)
                ├─► console stream (--format console)
                ├─► table / json / ndjson
                └─► engine ──► check adapters
                                  ├─ npm / pypi / crates / ...
                                  ├─ domain (DNS + RDAP + WHOIS)
                                  └─ cache (bbolt)
```

引擎与 UI 无关。它通过带缓冲的 channel 发出 `started` / `settled` 事件；TUI 通过 `tea.Cmd` 桥逐条读取，并按 `runID` 丢弃过期运行的事件。console 渲染器保留分组行，并随结果落地原位填充。每个 checker 声明自己的并发数、超时、优先级和缓存 TTL。

项目结构：

```
cmd/nameary          入口
internal/app         组装、规划、摘要
internal/engine      调度、重试、事件
internal/check       checker 契约 + HTTP 辅助
internal/check/adapters  注册表适配器
internal/check/domain    DNS、RDAP、WHOIS
internal/cache       bbolt 缓存
internal/tui         Bubble Tea model、view、动画
internal/render      table / JSON / NDJSON
internal/cli         Cobra 命令与旗标
internal/config      TOML 配置与预设
```

## 项目

- [贡献指南](CONTRIBUTING.md)
- [治理](GOVERNANCE.md)与[维护者](MAINTAINERS.md)
- [行为准则](CODE_OF_CONDUCT.md)
- [安全策略](SECURITY.md)与[安全审计](docs/security-audit.md)
- [威胁模型](docs/threat-model.md)与[保证案例](docs/assurance-case.md)
- [支持](SUPPORT.md)
- [路线图](ROADMAP.md)
- [分发计划](docs/distribution.md)
- [开源成熟度](docs/open-source-maturity.md)
- [更新日志](CHANGELOG.md)

## 范围与注意事项

- 可用性**不等于**商标清查。请使用 USPTO/EUIPO 检索。
- 不显示域名价格：没有稳定的免费注册商定价 API。详情视图改为链接到来源。
- Go module 不作为裸名检查（module 是完整路径）；请改用 GitHub 句柄目标。
- 按主机尊重限流，但各平台仍可能变更或封锁流量。这类情况显示为 `unknown`，而不是 `available`。

## 许可证

Apache-2.0
