# pandoc-tui

把常用的 pandoc 命令搬进终端界面。

[English](README.md)

pandoc 本身非常强大，但它的命令行更像一个工具箱：想把 Markdown 转成一份
排版规矩的 Word 文档，你得记住 `--reference-doc`、`--toc-depth`、
`--highlight-style` 以及一堆一年只用两次的参数。pandoc-tui 把这条流程放到
一个界面上——选转换、选文件、选样式，执行前先把完整的 pandoc 命令念给你听，
然后退到一边。

```
  pandoc-tui                                                          pandoc 3.10.2

  ╭─ Conversion  Markdown -> Word (.docx) ──────────────────────────────────────╮
  │ Convert a Markdown file into a Word document that follows a reference style.│
  │ 5 conversions are planned for later — press ? to see the roadmap           │
  ╰────────────────────────────────────────────────────────────────────────────╯

  ╭─ Source ────────────────────────────────────────────────────────────────────╮
  │ ▸ Input              ~/notes/quarterly-report.md                            │
  │   Output             ~/notes/quarterly-report.docx                          │
  ╰────────────────────────────────────────────────────────────────────────────╯

  ╭─ Style ─────────────────────────────────────────────────────────────────────╮
  │   Reference          House style                                            │
  │                      ~/.config/pandoc-tui/styles/House style.docx           │
  ╰────────────────────────────────────────────────────────────────────────────╯

  ╭─ Options ───────────────────────────────────────────────────────────────────╮
  │   Table of contents  on  space to toggle                                    │
  │   TOC depth          3  ‹ ›                                                 │
  │   Number sections    on  space to toggle                                    │
  │   Code highlight     pygments  ‹ ›                                          │
  │   Title              Quarterly report                                       │
  ╰────────────────────────────────────────────────────────────────────────────╯

  ╭─ pandoc command ────────────────────────────────────────────────────────────╮
  │ pandoc --from=markdown --to=docx --standalone ~/notes/quarterly-report.md    │
  │ --output=~/notes/quarterly-report.docx --reference-doc=~/.config/.../House   │
  │ style.docx --toc --toc-depth=3 --number-sections                            │
  ╰────────────────────────────────────────────────────────────────────────────╯

  ↑↓ move   ←→ change   enter choose   ctrl+r convert   ? help   q quit
```

## 当前状态

原型阶段，但功能是完整的。目前只接通了一条转换链路：

| 源格式   | 目标格式         | 说明                                 |
| -------- | ---------------- | ------------------------------------ |
| Markdown | Word (`.docx`)   | 支持参照样式、目录、代码高亮          |

架构是按「以后会有更多转换」来设计的：新增一条转换只需要写一个实现接口的
文件，界面代码一行都不用改。见[扩展一条转换](#扩展一条转换)。

## 运行要求

- `PATH` 里有 pandoc 3.x（找不到时程序会告诉你怎么办）
- 支持 UTF-8 的终端
- macOS / Linux / Windows

## 安装

```sh
go install github.com/WillHouMoe/pandoc-tui@latest
```

或者从源码构建：

```sh
git clone https://github.com/WillHouMoe/pandoc-tui
cd pandoc-tui
make build      # 产物在 bin/pandoc-tui
```

## 使用

```sh
pandoc-tui
```

| 按键              | 作用                                 |
| ----------------- | ------------------------------------ |
| `↑` `↓` / `k` `j` | 上下移动光标                         |
| `←` `→` / `h` `l` | 改值、拨开关                         |
| `enter`           | 选文件、进样式库、切换某个选项       |
| `b`               | 浏览文件                             |
| `ctrl+r`          | 开始转换                             |
| `?`               | 帮助，含路线图                       |
| `q`               | 退出                                 |

常用参数：

```sh
pandoc-tui --pandoc /opt/homebrew/bin/pandoc   # 指定 pandoc 路径
pandoc-tui --styles ~/Documents/docx-styles    # 把样式放在项目目录里
pandoc-tui --paths                             # 打印配置和样式目录位置
```

## 样式

这是这个项目真正值得做的部分。所谓「样式」就是一份 Word 参照文档：
pandoc 自带的 `reference.docx`，由你在 Word、Pages 或 LibreOffice 里改出来。
输出文档里除了正文之外的一切——字体、标题字号、行距、代码块的样式——都来自
这个文件。

在界面里：

1. 转换界面把光标移到 **Reference** 行，按 `enter` 进入样式库。
2. 按 `n` 新建样式。pandoc-tui 会复制 pandoc 本来就会用的那份参照文档，
   并直接用你的编辑器打开它。
3. 改完保存，回到界面，按 `enter` 选用。
4. `i` 导入已有的 `.docx`；`r` 重命名，`d` 删除，`o` 打开，`f` 在文件管理器里定位。

样式默认放在用户配置目录（Linux 是 `~/.config/pandoc-tui/styles`，macOS 是
`~/Library/Application Support/pandoc-tui/styles`）。想跟项目放一起，就用
`--styles` 指过去。

## 扩展一条转换

界面上画出来的所有东西都来自 `internal/convert` 里的注册表。写一个实现
`Converter` 的类型，注册进去，这条转换就自动有了自己的选项表单、文件过滤
和命令预览。

```go
// internal/convert/markdown_html.go
package convert

type markdownToHTML struct{}

func (markdownToHTML) ID() string              { return "markdown-html" }
func (markdownToHTML) Label() string           { return "Markdown -> HTML" }
func (markdownToHTML) Summary() string         { return "A standalone HTML5 page." }
func (markdownToHTML) From() Format            { return Markdown }
func (markdownToHTML) To() Format              { return HTML }
func (markdownToHTML) WantsReferenceDoc() bool { return false }

func (markdownToHTML) Options() []Option {
	return []Option{
		{Key: "css", Label: "Stylesheet", Help: "Path to a CSS file.",
			Kind: KindText, Flag: "--css"},
		{Key: "embed", Label: "Embed resources", Kind: KindToggle,
			Default: "true", Flag: "--embed-resources"},
	}
}

func (markdownToHTML) Validate(req Request) error {
	if err := CheckInput(req.Input); err != nil {
		return err
	}
	if req.Output == "" {
		return ErrNoOutput
	}
	return nil
}

func (c markdownToHTML) Args(req Request) ([]string, error) {
	args := BaseArgs(req, Markdown, HTML, "--standalone")
	for _, o := range c.Options() {
		args = append(args, o.Args(req.Value(o))...)
	}
	return args, nil
}
```

然后注册进去：

```go
func Default() *Registry {
	return NewRegistry(
		markdownToDOCX{},
		markdownToHTML{},   // <- 就这一行
	)
}
```

界面不用动：`Options()` 变成表单，`From()` / `To()` 决定文件过滤和推导出的
输出文件名，`Args()` 喂给命令预览。

## 代码结构

```
main.go                 装配：找 pandoc、读配置、启动程序
internal/convert/       扩展点：格式、选项、转换器
internal/pandoc/        定位 pandoc、流式执行、导出 reference.docx
internal/style/         样式库，本质就是一个文件夹
internal/config/        值得被记住的那几个选择
internal/ui/            Bubble Tea 界面，一个文件一个屏幕
```

界面从不直接调用 pandoc，它只渲染注册表暴露出来的东西。这是「加一条转换」
不会变成「做一个界面项目」的原因。

## 路线图

按大致顺序，架构已经为这些留好位置：

- Markdown -> HTML（需要 HTML writer 的选项）
- Markdown -> EPUB（需要封面和元数据处理）
- Markdown -> PDF（需要探测 LaTeX 引擎）
- Word -> Markdown（需要导出图片）
- 格式矩阵，替换掉现在写死的「源 -> 目标」一对

在界面里按 `?` 能看到同一份列表。

## 开发

```sh
make check      # gofmt、go vet、go test
make preview    # 把每个界面按纯文本打印出来，不需要终端
```

测试用例驱动的是真实的程序：用本机的 pandoc 创建样式，走界面同一条代码路径
转换一个 Markdown 文件，再检查生成的 `.docx`。需要 pandoc 的用例在没装
pandoc 时会自动跳过，所以空仓库也能跑测试。

## 许可

MIT，见 [LICENSE](LICENSE)。
