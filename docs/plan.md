# nameary — 设计与实施方案

> 一句话：`nameary acme` 回车后 100ms 内出现界面，结果像日志一样逐条“落”下来。用户永远不需要盯着一个转圈等全部完成。

## 0. 产品判断

这类工具真正难的不是“并发发请求”，而是三件事：

1. **开始得快**：首屏不能等任何网络请求。
2. **结果可信**：不能把超时、429、404 都当成“可用”。域名尤其不能把“没查到”说成“一定能注册”。
3. **过程好看但不闹**：动效服务于“正在发生什么”，不能每帧全屏重排、不能闪、不能抢焦点。

建议用三个指标约束实现：

| 指标 | 目标 | 含义 |
|---|---|---|
| TTFB（首帧） | < 100ms | 进程启动到界面出现 |
| TTFR（首条结果） | < 400ms | 第一条真实结果出现 |
| TTLR（全部结束） | 典型 < 3s | 最坏由单请求 timeout 兜底 |

产品形态建议：**单屏实时列表**，不做向导式多步。`nameary acme` 直接开跑；无参数时才显示输入页。

---

## 1. 体验设计

### 1.1 启动路径

> 2026-11 调整：默认 TTY 使用交互式 TUI；渐进式非交互输出通过 `--format console` 获取。

```bash
nameary                      # 输入页：logo + 输入框 + preset 提示
nameary acme                 # 跳过输入页，立即开始
nameary check acme --format json    # 无 TUI，脚本/CI 使用
nameary acme --preset indie  # 指定检查范围
```

- stdout 是 TTY 且未指定 `--format json/table/console` 时，默认进 TUI。
- 非 TTY 自动降级为流式纯文本（或 NDJSON），不启动 Bubble Tea。
- 运行中按 `r` 或重新输入：取消旧 run，旧事件按 `runID` 丢弃，不会串结果。
- `q` / `ctrl+c`：取消所有 in-flight 请求并退出；已返回的结果保留在终端（可选 `--keep` 打印摘要）。

### 1.2 主界面

```text
╭─ nameary ────────────────────────────────────────────────────╮
│                                                              │
│   acme                                    12 / 47   ◐        │
│   ████████████████░░░░░░░░░░░░░░░░░░░░░░  25%                │
│   9 available   ·   3 taken   ·   0 unknown                  │
│                                                              │
│   DOMAINS                                           3 / 8    │
│   ✓  acme.com        available    ~$12/yr     rdap ↗         │
│   ●  acme.io         taken        1995        whois ↗        │
│   ◐  acme.dev        checking…                               │
│   ◌  acme.app        queued                                  │
│                                                              │
│   PACKAGES                                          2 / 6    │
│   ✓  npm    acme     available               registry ↗      │
│   ●  pypi   acme     taken        v2.1.0     pypi ↗          │
│   ◐  crates acme     checking…                               │
│                                                              │
│   ↑↓ move   ⏎ details   / filter   a available   q quit      │
╰──────────────────────────────────────────────────────────────╯
```

设计要点：

- **所有行在启动时一次性以骨架态出现**，而不是结果到了才插入。用户立刻知道“要查 47 项”。
- 每行独立状态：`queued → checking → settled`，行内 spinner 共享同一个 tick，不每行开一个定时器。
- **不按状态实时重排**。结果落下来时行位置固定，只做一次高亮闪烁；否则用户正在看的行会跳走。
- 分组顺序固定：Domains → Packages → Handles。组内按用户传入的 TLD / target 顺序。
- 底部 footer 只显示 4–6 个当前可用快捷键，`?` 打开完整帮助。
- 宽屏显示元数据（版本、下载量、注册年份），窄屏只留名称 + 状态 + 延迟。
- 右侧不常驻详情栏，避免双栏在 80 列下拥挤；`⏎` 打开覆盖层，`esc` 关闭。

### 1.3 详情覆盖层

```text
╭─ acme.dev ───────────────────────────────────────────╮
│  ● taken · likely · 412ms · cached 3h                │
│                                                      │
│  registrar   Namecheap, Inc.                         │
│  created     2018-04-12                              │
│  expires     2027-04-12                              │
│  status      clientTransferProhibited                │
│  nameservers ns1.example.net                         │
│                                                      │
│  source      https://rdap.org/domain/acme.dev  ↗     │
│                                                      │
│  这是注册局 RDAP 的记录。域名“可用”仅代表未查到注册   │
│  记录，不代表可注册或价格；保留域名/溢价域名请以注册   │
│  商结账页为准。                                       │
╰──────────────────────────────────────────────────────╯
```

- 详情用 `viewport` 滚动；内容只在打开时渲染一次并缓存，不参与每帧重绘。
- 来源链接用 OSC 8 hyperlink，`o` 用系统浏览器打开。
- 域名详情必须带“可用 ≠ 可注册”的免责声明；这是产品可信度的一部分。

### 1.4 交互键位

| 键 | 行为 |
|---|---|
| `↑/k` `↓/j` | 移动 |
| `g/G` `pgup/pgdn` | 顶部/底部 |
| `⏎` | 打开详情 |
| `esc` | 关闭覆盖层 / 取消当前 run |
| `/` | 过滤（名称、target、状态） |
| `a` | 只看 available |
| `s` | 切换排序（分组 / 状态 / 延迟） |
| `c` | 复制选中项名称或 URL |
| `o` | 打开来源链接 |
| `r` | 重跑当前项（`R` 全部重跑） |
| `e` | 导出 JSON / Markdown |
| `?` | 帮助 |
| `q` / `ctrl+c` | 退出 |

鼠标可选支持（`View.MouseMode`），但所有操作都必须有键盘路径。

---

## 2. 技术选型

### 2.1 Charm v2 全家桶

截至 2026-11，Charm 的 v2 已经稳定，导入路径改为 `charm.land/*/v2`，且 **要求 Go 1.26+**。

| 库 | 用在哪 | 为什么 |
|---|---|---|
| `charm.land/bubbletea/v2` | 运行时 | Cursed Renderer、声明式 `View`、alt screen、原生进度条、剪贴板、背景色探测 |
| `charm.land/bubbles/v2/textinput` | 名称/过滤输入 | 成熟的编辑、粘贴、光标处理 |
| `charm.land/bubbles/v2/viewport` | 详情/帮助滚动 | 不用自己处理换行与滚动 |
| `charm.land/bubbles/v2/spinner` | 全局 spinner | 只维护一个模型，所有 checking 行共享帧 |
| `charm.land/bubbles/v2/progress` | 主进度条 | 自带 spring 动画、渐变、`IsAnimating()`，不用手写缓动 |
| `charm.land/bubbles/v2/help` + `key` | footer / 帮助 | 键位声明与帮助生成 |
| `charm.land/lipgloss/v2` | 布局、边框、颜色 | `Blend1D` 渐变、`Canvas`/`Layer` 覆盖层合成 |
| `charm.land/harmonica` | 覆盖层/摘要 spring | 只用于 2–3 处大过渡，不滥用 |
| `charm.land/glamour/v2` | 帮助页 / Markdown 导出 | 可选；详情页不用它，避免每帧渲染 |
| `charm.land/huh/v2` | `nameary config init` | 只在配置向导里用，不进主流程 |
| `charm.land/log/v2` | `--debug` 文件日志 | TUI 模式下 stdout 必须干净 |
| `github.com/charmbracelet/x/ansi` | 截断/宽度 | 处理 CJK、emoji、超链接 |
| `github.com/charmbracelet/x/term` | TTY 检测 | 决定是否进 TUI |
| `github.com/charmbracelet/colorprofile` | 色彩降级 | truecolor → 256 → 16 |
| `charm.land/x/exp/teatest/v2` | TUI 黄金测试 | 帧级快照 |
| `vhs` | README 演示 GIF | 工具，不是 Go 依赖 |
| `wish` | 未来的 `nameary serve` | v1 不做，不引入 |

**刻意不用的库**：

- `bubbles/list`：自带标题、分页、过滤，和“固定行 + 流式状态 + 分组”冲突，定制成本高于手写。
- `bubbles/table`：适合静态表格，不适合每行独立动画与状态。
- `bubbles/paginator`、`filepicker`：本产品用不上。
- `gum`：是给 shell 脚本用的 CLI，不是库。
- `glow`：是 Markdown 阅读器，不是库；只需要 `glamour`。
- `wish`：SSH 托管是另一个产品形态，v1 不加。

> “把所有 Charm 库都用上”不是目标。每个库都应该有明确职责；否则二进制和认知负担都会膨胀。

### 2.2 非 Charm 依赖

| 依赖 | 用途 |
|---|---|
| `github.com/spf13/cobra` | 命令、flag、completion |
| `golang.org/x/sync/semaphore` | 全局/每主机并发限制 |
| `golang.org/x/net/idna` | 域名 punycode |
| `golang.org/x/net/publicsuffix` | 判断完整域名与 eTLD+1 |
| `go.etcd.io/bbolt` | 纯 Go 缓存，无 CGO |
| `github.com/BurntSushi/toml` | 配置文件 |

不引入 Viper：配置项不多，flag > env > TOML > default 的手写优先级更可控。

---

## 3. 架构

```text
                    ┌─────────────────────────────┐
                    │        cmd/nameary          │
                    │  Cobra: check/config/...    │
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │         internal/app        │
                    │  选 TUI / plain / JSON 模式  │
                    └───────┬─────────────┬───────┘
                            │             │
              ┌─────────────▼───┐   ┌─────▼──────────────┐
              │  internal/tui   │   │ internal/render    │
              │  Bubble Tea v2  │   │ table / json / ndjson│
              └────────┬────────┘   └─────────┬──────────┘
                       │  Event stream        │
                       └──────────┬───────────┘
                                  │
                       ┌──────────▼───────────┐
                       │   internal/engine    │
                       │ 调度 / 限流 / 重试 /  │
                       │ 取消 / 事件通道       │
                       └──────────┬───────────┘
                                  │
              ┌───────────────────┼────────────────────┐
              │                   │                    │
        ┌─────▼─────┐      ┌──────▼──────┐      ┌──────▼──────┐
        │  check/*  │      │   cache     │      │  domain     │
        │  npm/pypi │      │   bbolt     │      │  DNS + RDAP │
        │  crates…  │      └─────────────┘      └─────────────┘
        └───────────┘
```

依赖方向：`tui` 和 `render` 依赖 `engine` 与 `check`；`engine` 不依赖任何 UI；`check` 不依赖 `engine`。这样 TUI 可以完全用假 engine 测试，headless 模式也能复用同一套检查器。

### 3.1 核心类型

```go
package check

type Target string

const (
	TargetNPM     Target = "npm"
	TargetPyPI    Target = "pypi"
	TargetCrates  Target = "crates"
	TargetDomains Target = "domains"
	TargetGitHub  Target = "github"
)

type Status int

const (
	StatusQueued Status = iota
	StatusChecking
	StatusAvailable
	StatusTaken
	StatusUnknown
	StatusError
	StatusSkipped
)

type Confidence int

const (
	Certain Confidence = iota // 注册表明确返回 404
	Likely                    // RDAP 404，但可能有保留/溢价
	Low                       // 启发式 / HTML 兜底
)

type Result struct {
	Status     Status
	Confidence Confidence
	Query      string            // 规范化后的查询词
	Summary    string            // "v2.1.0 · 25M/wk"
	Fields     map[string]string // 详情页字段
	SourceURL  string
	FetchedAt  time.Time
	Latency    time.Duration
	Cached     bool
	Err        error
}

type Checker interface {
	Target() Target
	Label() string
	Normalize(raw string) (string, error)
	Check(ctx context.Context, q string) (Result, error)
	Homepage(q string) string
}
```

关键点：`Result` 不是 `bool`。UI 必须能区分“确定可用”“可能可用”“超时”“被限流”“名字非法”。

### 3.2 Job 与 Event

```go
type Job struct {
	ID       string // target + ":" + normalized query
	Target   check.Target
	Raw      string
	Query    string
	Priority int
}

type Event struct {
	RunID  uint64
	JobID  string
	Kind   EventKind // EventStarted | EventSettled
	Result check.Result
	At     time.Time
}
```

- TUI 在启动时根据 `[]Job` 直接构造所有行，不需要 engine 发“排队”事件。
- engine 只发 `started` / `settled` 两类事件，消息量等于 target 数，80 项大约 160 条，完全不需要节流。
- 如果未来支持 500+ target，再加 16ms 合并 drain；v1 不做过度设计。

### 3.3 异步事件流（核心）

engine 内部跑 goroutine 池，通过带缓冲 channel 输出事件；TUI 用“一次读一条”的 `tea.Cmd` 桥接：

```go
func waitEvent(ch <-chan engine.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return streamDoneMsg{}
		}
		return eventMsg(ev)
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.startRun,                 // 启动 goroutine，返回 nil
		waitEvent(m.events),
		tea.RequestBackgroundColor,
		m.spinner.Tick,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case eventMsg:
		if msg.RunID != m.runID {
			return m, waitEvent(m.events) // 丢弃旧 run 的迟到事件
		}
		m.apply(msg)
		return m, waitEvent(m.events)     // 继续读下一条
	case streamDoneMsg:
		m.finish()
		return m, nil
	}
	return m, nil
}
```

为什么不用 `tea.Program.Send` 从 goroutine 直接推？

- channel + Cmd 的读取顺序天然稳定，测试时可以用假 channel 精确控制帧。
- 不会出现“程序已退出但 goroutine 还在 Send”的泄漏。
- 未来换 headless 输出时，engine 完全不用改。

### 3.4 调度、并发、限流

```go
func (e *Engine) Run(ctx context.Context, runID uint64, jobs []Job) <-chan Event {
	out := make(chan Event, 256)
	work := make(chan Job)

	var wg sync.WaitGroup
	for i := 0; i < e.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range work {
				e.runOne(ctx, runID, j, out)
			}
		}()
	}

	go func() {
		defer close(work)
		for _, j := range jobs { // jobs 已按 Priority 排序
			select {
			case work <- j:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}
```

- **优先级**：Domains 先发（最慢、最受关注），随后 npm/PyPI（最快出结果），再 GitHub，最后长尾。这样首条结果通常来自 npm/PyPI，而域名不会拖到最后才开始。
- **全局并发**：默认 16；`--concurrency` 可调。
- **每主机并发**：默认 4；crates.io 1；Packagist 2；GitHub 1（无 token）/4（有 token）；RDAP 每个注册局 2–4。
- **抖动**：请求前随机 0–250ms，避免同一毫秒打爆一个注册局。
- **超时**：单请求 8s（Packagist 12s），连接 3s；整体不设 deadline，保证渐进输出。
- **重试**：仅对 429/502/503/504 和临时网络错误，最多 2 次，指数退避 + 抖动；尊重 `Retry-After`，上限 5s。
- **不要重试 404**：404 是“可用”的明确信号，重试只会放大延迟。

### 3.5 取消与重跑

- 每次 run 持有 `context.WithCancel`；`r` / 新输入 / `q` 调用 cancel。
- `runID` 单调递增；所有事件携带 `runID`，TUI 丢弃过期事件。
- engine 的 worker 在每次 HTTP 请求和 channel send 上都 `select ctx.Done()`，保证取消后 goroutine 能退出。
- 用 `go.uber.org/goleak` 在测试里守住“取消后无泄漏”。

### 3.6 缓存

- bbolt，路径 `os.UserCacheDir()/nameary/cache.db`。
- key：`v1:<target>:<normalized query>`；value：`Result` JSON + `fetchedAt`。
- TTL：域名 6h，包注册表 24h，GitHub 12h。
- 只缓存确定结果（available/taken），不缓存 error/timeout/429。
- 命中缓存时 `Cached=true`，UI 显示 `cached 3h`；`--cache-mode refresh` 绕过。
- `r` 重跑选中项时强制刷新。

---

## 4. 检查源矩阵

### 4.1 首批支持（v1）

> 2026-11 追加：Maven Central、Conda、CocoaPods、AUR、Snap Store、Chocolatey、Hackage、JSR，共 20 个 target。

| Target | 请求 | 可用信号 | 可展示元数据 | 备注 |
|---|---|---|---|---|
| npm | `GET https://registry.npmjs.org/{name}` | 404 | latest、description、time | 小写；scoped 需 URL 编码 `@scope%2Fname` |
| PyPI | `GET https://pypi.org/pypi/{name}/json` | 404 | version、summary、author | PEP 503 规范化：`[-_.]+` → `-`，小写 |
| crates.io | `GET https://crates.io/api/v1/crates/{name}` | 404 | max_version、downloads、description | 必须带 UA；建议 1 req/s；`-`/`_` 需额外提示 |
| RubyGems | `GET https://rubygems.org/api/v1/gems/{name}.json` | 404 | version、downloads | 大小写不敏感 |
| NuGet | `GET https://api.nuget.org/v3-flatcontainer/{lower}/index.json` | 404 | versions | 302 到 CDN，跟随后看状态 |
| Docker Hub | `GET https://hub.docker.com/v2/users/{name}/`、`/orgs/{name}/` | 404 | 仓库数、拉取量 | 检查 namespace，不只看镜像名 |
| Homebrew | `GET https://formulae.brew.sh/api/formula/{name}.json` + `/cask/{name}.json` | 两者都 404 | version、desc | formula 与 cask 都要查 |
| Packagist | `GET https://packagist.org/packages/{vendor}/{name}.json` | 404 | downloads、description | 需要 `vendor/name`；很慢，低优先级 + 12s 超时 |
| Hex | `GET https://hex.pm/api/packages/{name}` | 404 | latest、downloads | |
| pub.dev | `GET https://pub.dev/api/packages/{name}` | 404 | latest | 小写下划线 |
| GitHub | `GET https://api.github.com/users/{name}` / `/repos/{name}/{name}` | 404 | followers、public_repos | 无 token 60/h；默认 1 req/s + 缓存；有 token 才放开 |
| Domains | DNS NS + RDAP | 见 4.3 | registrar、created、expires、status | 两阶段；可用只标 `likely` |

### 4.2 暂不做

- **Go modules**：模块是完整路径（`github.com/foo/bar`），裸名没有意义。用 GitHub target 代替；如果用户传入完整 module path，再走 `proxy.golang.org/{path}/@v/list`。
- **JSR**：API 仍不稳定，v1 不做。
- **社交平台**：X/Instagram 反爬严重，可用性判断不可靠；Bluesky 可后续加。
- **商标**：需要付费/专业数据库，v1 只在详情页给 USPTO/EUIPO 搜索链接，不做判断。
- **域名实时价格**：没有稳定免费的注册商 API；v1 只给注册商链接，不展示价格（避免误导）。

### 4.3 域名检查：DNS + RDAP 两阶段

```text
输入 acme.dev
  │
  ├─ 1. LookupNS(acme.dev)
  │      ├─ 有 NS  → taken（快，~50ms，直接返回）
  │      └─ NXDOMAIN / 无 NS / 超时 → 进入 2
  │
  ├─ 2. IANA RDAP bootstrap（缓存 7 天）
  │      找到 .dev 的 RDAP base；找不到则用 https://rdap.org/domain/{domain}
  │
  └─ 3. GET {base}/domain/acme.dev
         ├─ 200 → taken，解析 registrar/created/expires/status
         ├─ 404 → available，confidence=likely
         ├─ 429 → unknown，标记 rate-limited，可重试
         └─ 5xx/超时 → unknown/error，不猜
```

实现细节：

- **DNS 只用于快速判定“已注册”**。有 NS 的域名一定已注册；没有 NS 不代表未注册（可能未配置解析），所以“可用”必须由 RDAP 确认。
- **RDAP bootstrap** 用 `https://data.iana.org/rdap/dns.json`，按 TLD 找注册局 base，避免 `rdap.org` 的 302 往返；bootstrap 文件本地缓存 7 天。
- **RDAP 404** 只能得出“注册局没有记录”，保留域名/溢价域名仍可能存在，所以标 `likely`，详情页给出注册商链接与免责声明。
- **IDN**：`idna.Lookup.ToASCII` 转 A-label 再查，展示时保留 U-label。
- **完整域名输入**：用 `publicsuffix` 判断；`acme.co.uk` 不再追加 TLD；`acme` 才按 `--tlds` 展开。
- **DNS 被污染/拦截**：可选 DoH 兜底（`https://dns.google/resolve?name=...&type=NS`），默认不开，避免额外依赖。
- **不做 WHOIS**：端口 43 在部分网络被墙，且各注册局格式差异大；RDAP 覆盖不足时明确返回 `unknown`，比瞎猜好。

### 4.4 名称规范化

| Target | 规则 |
|---|---|
| npm | trim、小写；scoped 保留 `@scope/name`；拒绝 `.`/`_` 开头；长度 ≤ 214 |
| PyPI | `regexp.MustCompile("[-_.]+").ReplaceAllString(name, "-")` 后小写 |
| crates.io | 小写；查询原样；若含 `_`，额外查 `-` 变体，提示“变体已被占用” |
| RubyGems | 小写 |
| NuGet | 小写 |
| Docker Hub | 小写，仅 `[a-z0-9._-]` |
| Homebrew | 小写，`[a-z0-9@+._-]` |
| Hex | 小写，`[a-z0-9_]` |
| pub.dev | 小写，`[a-z0-9_]` |
| GitHub | 字母数字与 `-`，≤39，首尾不能 `-`，不能连续 `--` |
| Domain | 小写、去尾点；LDH 规则；IDN 转 punycode |

UI 的详情页要显示“实际查询词”，避免用户以为查的是原始输入。

---

## 5. TUI 实现

### 5.1 Model 结构

```go
type model struct {
	ctx    context.Context
	cancel context.CancelFunc

	width, height int
	phase         phase // input | running | done
	name          string
	input         textinput.Model
	filter        textinput.Model
	filtering     bool

	runID  uint64
	jobs   []engine.Job
	rows   []row
	byID   map[string]int
	events <-chan engine.Event

	cursor  int
	offset  int
	onlyOK  bool
	sortBy  sortMode
	detail  *detailView
	help    help.Model
	keys    keyMap
	spinner spinner.Model
	prog    progress.Model

	stats     stats
	startedAt time.Time
	anim      animState
}
```

`row` 持有 `job`、`result`、`startedAt`、`settledAt`、`flash`，渲染时只读。

### 5.2 Update 流程

```text
KeyMsg ──► 输入页：更新 textinput；Enter 启动 run
       └─► 运行页：导航/过滤/详情/重跑/退出

eventMsg ──► 更新对应 row；更新 stats；触发 settle 动画
         └─► 继续 waitEvent

streamDoneMsg ──► phase=done；进度条到 100%；摘要入场
```

### 5.3 渲染性能

- `View()` 只渲染**可见行**（约 20–30 行），不是全部 200 行。
- 行字符串按 `(width, status, cached, elapsedBucket)` 缓存；状态不变就不重算。
- 用 `strings.Builder` 拼装；避免每帧 `fmt.Sprintf` 大量小字符串。
- 不每帧调用 `lipgloss.Place` / `JoinVertical` 做整屏布局；外层边框只在 resize 时计算。
- `lipgloss.Canvas` + `Layer` 只用于详情覆盖层：底层列表 + 半透明遮罩 + 居中面板，一次合成。
- 动画 tick 只在需要时运行；静止时 0 帧。
- `View.AltScreen = true`；`View.WindowTitle` 显示 `nameary · acme · 12/47`；`View.ProgressBar` 用 OSC 9;4 同步终端任务栏进度。
- 程序状态用 `View.ProgramStatus` 标记 working/done，支持该协议的终端会显示。

### 5.4 布局断点

| 宽度 | 布局 |
|---|---|
| ≥ 100 | 名称、状态、元数据、延迟、来源全部显示 |
| 70–99 | 隐藏来源与部分元数据，保留状态 + 延迟 |
| < 70 | 单列紧凑：状态 glyph + 名称 + 状态文字；详情仍可用 |
| < 50 | 隐藏分组标题，改为行前缀 `D`/`P`/`H` |

高度 < 12 时只显示摘要 + 当前行，避免边框挤爆。

### 5.5 主题与可访问性

- 颜色通过 `tea.BackgroundColorMsg.IsDark()` 选择 light/dark 两套；不要假设终端背景。
- 调色板：
  - accent `#5EEAD4`（teal）
  - available `#34D399`（emerald）
  - taken `#FB7185`（rose）
  - unknown `#FBBF24`（amber）
  - error `#F87171`
  - muted `#64748B`
- 状态永远同时有 **glyph + 文字**，不能只靠颜色区分。
- `NO_COLOR` / `CLICOLOR=0` / `TERM=dumb` → 单色；`--style ascii` → ASCII glyph（`+ x o . ! X`）。
- `--style calm` / `NAMEARY_REDUCED_MOTION=1` → 关闭 shimmer、闪烁、spring，只保留 spinner 与进度。
- 不用 Nerd Font 图标；只用 Unicode 几何符号，保证默认字体可用。

---

## 6. 动效系统

### 6.1 原则

- 动效只回答“现在发生了什么”，不做纯装饰。
- 同一时刻最多一个 60fps 动画；静止时停止 tick。
- 首屏不做长入场动画；用户是来查名字的，不是看片头。
- 所有动画可被 `--style calm` 或 `--style plain` 关闭。

### 6.2 动效清单

| 动效 | 时长 | 触发 | 实现 |
|---|---|---|---|
| Logo 渐变扫过 | 900ms，一次 | 输入页出现 | `lipgloss.Blend1D` + 逐字着色 |
| 骨架 shimmer | 1.2s 循环 | 有 queued/checking 行 | 一个 tick 驱动，移动高光带 |
| 行 spinner | 80ms/帧 | checking 行 | 共享 `spinner.Model` |
| 结果落定闪烁 | 350ms，一次 | settled | 背景色 alpha 衰减 + glyph 切换 |
| 计数器 | 250ms | stats 变化 | easeOutCubic 插值 |
| 进度条 | spring | 进度变化 | `bubbles/progress` 自带 spring |
| 详情覆盖层 | 180ms | `⏎` / `esc` | Canvas/Layer 位置 + 遮罩淡入 |
| 完成摘要 | 300ms | stream done | harmonica spring |
| 彩带（可选） | 800ms | 全部 available + `--celebrate` | 30 行粒子，自己写，不引第三方 |

### 6.3 tick 循环

```go
func (m model) tickIfNeeded() tea.Cmd {
	if !m.anim.active() {
		return nil
	}
	return tea.Tick(time.Second/60, func(t time.Time) tea.Msg {
		return frameMsg(t)
	})
}
```

`anim.active()` 的条件：

- 有行在 400ms 内刚 settle；
- 详情层正在打开/关闭；
- 摘要正在入场；
- 有 pending 行（shimmer）。

全部结束后返回 `nil`，CPU 归零。窗口失焦（`tea.BlurMsg`）时暂停动画，聚焦恢复。

### 6.4 不要做的动效

- 不要做逐行飞入/位移：结果流式到达时行会跳动，反而降低可读性。
- 不要在结果到达时自动排序：用户视线会丢。
- 不要全屏闪烁或反色：终端里非常刺眼。
- 不要每行独立 spinner tick：80 行会有 80 个定时器。

---

## 7. CLI、输出与配置

### 7.1 命令

```text
nameary [name] [flags]          # 默认：TTY 进 TUI，非 TTY 流式输出
nameary check <name> [flags]    # 明确的检查命令（脚本友好）
nameary config init|path        # 配置
nameary completion bash|zsh|fish
nameary version
```

### 7.2 Flags

```text
--preset indie|web|go|rust|all   检查范围预设
--targets npm,pypi,domains,...   显式指定 target
--tlds com,io,dev,ai             域名后缀
--concurrency 16                 全局并发
--timeout 8s                     单请求超时
--format auto|tui|console|table|json|ndjson  输出模式
--style auto|plain|ascii|mono|calm          色彩/字形/动效合并参数
--only-available                 只输出可用项
--cache-mode auto|off|refresh    缓存行为
--cache-ttl 12h                  覆盖缓存 TTL
--github-token string            也可用 GITHUB_TOKEN/GH_TOKEN
--debug                          写 debug 日志到缓存目录
```

### 7.3 输出模式

- **console**：`--format console` 的渐进式输出，先打印 logo、分组和骨架行，再逐条填充结果；TTY 下带动态状态行，终端过矮时回退为分组块输出。
- **TUI**：默认在 TTY 开启，也可用 `--format tui` 显式强制。
- **table**：带颜色的对齐表格，非 TTY 自动去色。
- **json**：全部结束后输出数组，适合 `jq`。
- **ndjson**：每 settle 一行，适合流式管道。

```json
{"name":"acme","target":"npm","status":"available","confidence":"certain","url":"https://www.npmjs.com/package/acme","latency_ms":312}
```

进度、错误摘要写 stderr；结果写 stdout，保证管道干净。

### 7.4 退出码

| 码 | 含义 |
|---|---|
| 0 | 检查完成（无论结果是否可用） |
| 1 | 引擎/配置错误 |
| 2 | 用法错误 |
| 3 | 指定了 `--fail-if-taken` 且存在 taken |

### 7.5 配置

`~/.config/nameary/config.toml`：

```toml
preset = "indie"
tlds = ["com", "io", "dev", "app", "ai", "co"]
targets = ["domains", "npm", "pypi", "github"]
concurrency = 16
timeout = "8s"
cache_ttl = "24h"
celebrate = false

[github]
token_env = "GITHUB_TOKEN"
```

优先级：flag > env > config > default。`nameary config init` 用 huh 表单生成；`nameary config path` 打印路径。

### 7.6 Preset

| Preset | 内容 |
|---|---|
| `indie`（默认） | domains `com/io/dev/app/ai/co` + npm + pypi + github |
| `web` | npm + pypi + domains `com/io/dev/app` + github |
| `go` | github + domains `com/io/dev` + homebrew |
| `rust` | crates + github + domains `com/io/dev` |
| `all` | 全部（慢，适合最终确认） |

---

## 8. 可靠性与正确性

### 8.1 错误分类

```go
type HTTPError struct {
	Code int
	URL  string
}
```

| 情况 | 状态 | 是否重试 |
|---|---|---|
| 404 | available | 否 |
| 200 + 合法 body | taken | 否 |
| 429 | unknown（rate-limited） | 是，尊重 Retry-After |
| 502/503/504 | unknown/error | 是，最多 2 次 |
| 超时 | unknown | 是，1 次 |
| DNS 失败 | unknown | 否（除非 DoH 兜底） |
| 非法名称 | skipped | 否 |
| body 解析失败 | error | 否 |

**铁律：任何非 404 的异常都不能映射成 available。**

### 8.2 HTTP 客户端

- 全局复用一个 `http.Client`，`Transport.MaxIdleConnsPerHost = 8`，启用 HTTP/2。
- UA：`nameary/0.1 (+https://github.com/yufeiminds/nameary)`，带联系方式，方便注册局放行。
- 每主机 `semaphore.Weighted`；不要用一个全局并发数打所有站点。
- 对 `crates.io`、`Packagist`、GitHub 无 token 场景降低并发。

### 8.3 GitHub 配额

无 token 只有 60 req/h，很容易被限。策略：

1. 有 `GITHUB_TOKEN`/`GH_TOKEN` → API，4 并发，5000/h。
2. 无 token → 先试 HTML `HEAD https://github.com/{name}`（200 taken / 404 available），1 req/s，12h 缓存；失败再退回 API。
3. UI 在 GitHub 行显示 `token?` 提示；`--debug` 里记录剩余配额。

### 8.4 域名结果的可信度

- `taken`：DNS NS 或 RDAP 200，confidence `certain`。
- `available`：RDAP 404，confidence `likely`；详情页明确写“未查到注册记录”。
- `unknown`：RDAP 429/超时/不支持该 TLD；给出注册商链接，不猜。
- 注册商链接建议用中立的 `https://rdap.org/domain/{domain}` 作为证据，另加一个可配置的注册商链接。

---

## 9. 测试与演示

| 层级 | 方式 |
|---|---|
| Checker 单测 | `httptest.Server` + 固定 JSON；覆盖 200/404/429/超时/坏 body |
| 规范化 | 表驱动，覆盖 npm scoped、PyPI `-_.`、IDN、非法域名 |
| Engine | 假 checker + 延迟；断言事件顺序、优先级、取消、无泄漏（goleak） |
| TUI | `teatest/v2` 黄金帧：初始、部分结果、完成、过滤、窄屏、NO_COLOR |
| 动画 | 注入固定时钟，断言 tick 在静止时停止 |
| 集成 | `go test -race ./...`；VHS 录制 README GIF |
| 发布 | goreleaser：brew tap、deb/rpm、scoop、GitHub Releases |

VHS tape 示例（放 `demo/demo.tape`）：

```text
Output demo/nameary.gif
Set FontSize 14
Set Width 1100
Set Height 700
Type "nameary acme"
Enter
Sleep 3s
```

---

## 10. 目录结构

```text
nameary/
├── cmd/nameary/main.go
├── internal/
│   ├── cli/                 # Cobra 命令、flag、配置装载
│   │   ├── root.go
│   │   ├── check.go
│   │   ├── config.go
│   │   └── version.go
│   ├── app/                 # 选择 TUI / table / json
│   │   └── app.go
│   ├── engine/              # 调度、限流、重试、事件
│   │   ├── engine.go
│   │   ├── job.go
│   │   ├── event.go
│   │   └── limiter.go
│   ├── check/               # Checker 接口 + 各注册表适配
│   │   ├── checker.go
│   │   ├── registry.go
│   │   ├── npm/
│   │   ├── pypi/
│   │   ├── crates/
│   │   ├── rubygems/
│   │   ├── nuget/
│   │   ├── dockerhub/
│   │   ├── homebrew/
│   │   ├── packagist/
│   │   ├── hex/
│   │   ├── pubdev/
│   │   ├── github/
│   │   └── domain/          # DNS + RDAP bootstrap
│   ├── cache/               # bbolt
│   ├── tui/
│   │   ├── app.go           # tea.Program 启动
│   │   ├── model.go
│   │   ├── update.go
│   │   ├── view.go
│   │   ├── row.go
│   │   ├── detail.go
│   │   ├── theme.go
│   │   ├── anim.go
│   │   ├── keys.go
│   │   └── components/      # 可复用的小组件
│   ├── render/              # table/json/ndjson
│   └── config/
├── demo/demo.tape
├── docs/plan.md
├── go.mod
└── README.md
```

---

## 11. 里程碑

### M0 — 骨架能跑（1–2 天）

- Cobra 根命令 + `check` 子命令 + flags。
- `Checker` 接口、engine worker pool、事件 channel。
- 3 个 checker：npm、PyPI、`.com` RDAP。
- 纯文本流式输出；`--format json`。
- **验收**：`nameary check acme --format ndjson` 能逐条输出，超时不会误报 available。

### M1 — TUI 流式（2–3 天）

- Bubble Tea v2 单屏：输入页、行骨架、逐条状态、进度、footer。
- `waitEvent` 桥接、取消、runID。
- 详情覆盖层（先只显示原始字段）。
- **验收**：40 项检查首屏 <100ms，结果逐条出现，`q` 取消无泄漏。

### M2 — 全量检查器 + 缓存（3–4 天）

- 其余 registry + GitHub + 域名两阶段。
- 规范化、confidence、错误分类、重试、限流。
- bbolt 缓存、`--cache-mode refresh`、`cached` 标记。
- **验收**：所有 target 有单测；429/超时在 UI 中显示为 unknown 而非 available。

### M3 — 美化与交互（3–4 天）

- 主题、渐变、progress spring、settle 闪烁、shimmer、详情层动画。
- 过滤、只看可用、排序、复制、打开链接、导出。
- 窄屏/ASCII/NO_COLOR/reduced-motion。
- **验收**：黄金帧测试通过；静止时无 tick；80 列下不溢出。

### M4 — 发布（2 天）

- VHS 演示、README、goreleaser、brew tap。
- `--debug` 日志、GitHub token 提示、文档。
- **验收**：`brew install` 可用；CI 绿；release 产物含 darwin/linux/windows。

---

## 12. 风险与取舍

| 风险 | 影响 | 应对 |
|---|---|---|
| 域名“可用”被误解为“能注册” | 信任受损 | `likely` + 免责声明 + 注册商链接；不展示价格 |
| RDAP 覆盖不全/限流 | 结果 unknown | IANA bootstrap + rdap.org 兜底；明确 unknown，不猜 |
| GitHub 无 token 配额 | 大量 403 | HTML 兜底 + 缓存 + token 提示 |
| 注册表限流/封 UA | 检查失败 | 每主机并发、UA 带联系方式、退避、缓存 |
| Packagist 很慢 | 拖尾 | 低优先级 + 12s 超时 + 单独并发限制 |
| 动画耗电/卡顿 | 体验下降 | 静止停 tick；60fps 仅过渡；reduced-motion |
| 终端兼容性 | 乱码/颜色异常 | colorprofile 降级；ASCII 兜底；不用 Nerd Font |
| 名称规范化误判 | 假阳性/假阴性 | 每 target 独立 normalizer；详情显示实际查询词 |
| 库版本升级 | v2 API 变动 | 锁定 minor；teatest 黄金帧守住 UI |
| 商标/法律 | 超出范围 | 明确声明“不是商标检索” |

---

## 13. 待确认

1. 当前实现有 20 个 target（见 README）；是否还要继续加/砍（比如 Go module path、CPAN、JSR scope）？
2. GitHub 是否作为核心 target？如果常用，建议默认读 `GITHUB_TOKEN`，否则配额会很痛。
3. 域名是否需要展示价格？免费稳定源基本没有，建议 v1 只给注册商链接。
4. 默认 preset 用 `indie` 是否合适？还是默认只查 domains + npm + pypi，保持更快？
5. 是否要我在这个仓库直接开始 M0（Cobra + engine + npm/PyPI/RDAP + NDJSON）？

---

## 14. 落地结果（2026-11）

M0–M4 已实现：

- Charm v2（`charm.land/bubbletea/v2`、`bubbles/v2`、`lipgloss/v2`）+ Go 1.26。
- engine / checker / cache / TUI / headless render 分层完成。
- 20 个 target：domains、npm、PyPI、crates.io、RubyGems、NuGet、Docker Hub、Homebrew、Packagist、Hex、pub.dev、GitHub，以及 Maven Central、Conda、CocoaPods、AUR、Snap Store、Chocolatey、Hackage、JSR。
- 域名：DNS 快速判定 + IANA RDAP bootstrap + `rdap.org` 兜底 + ccTLD WHOIS fallback；RDAP 404 与“无 RDAP 服务”严格区分。
- 默认 TTY 输出：全屏交互式 TUI；`--format console` 先打印 logo/分组/骨架，再逐条填充结果，TTY 下带动态状态行；非 TTY 自动降级为 table。
- TUI（`--format tui`）：骨架先行、逐条 settle、per-host 限流、runID 取消、详情覆盖层、过滤/排序/导出、ASCII/NO_COLOR/reduced-motion。
- 缓存：bbolt，按 target TTL；`--cache-mode refresh` / `--cache-ttl`。
- 配置：TOML + `config init`（TTY 下用 huh 表单）。
- 质量：`go test ./...`、`go test -race ./...`、`go vet ./...`；VHS tape、goreleaser、GitHub Actions。
- 发布：goreleaser 已配置 darwin/linux/windows 与 deb/rpm/apk；Homebrew tap 尚未发布（需要单独的 `homebrew-tap` 仓库）。

与最初方案的差异：

- 没有引入 `glamour` 和 `teatest`：帮助页用 lipgloss 直接排版，TUI 测试用直接 View 快照 + 尺寸断言，依赖更少、反馈更快。
- `harmonica` 只用于详情覆盖层的 spring；主进度条直接用 `bubbles/progress` 自带的 spring。
- 增加 `--targets all`，以及“完整域名参数默认只查该域名”的行为。
- 增加 WHOIS fallback：`.co` 等没有公开 RDAP 的 TLD 不会因为“没有 RDAP 服务”被误判为可用。
