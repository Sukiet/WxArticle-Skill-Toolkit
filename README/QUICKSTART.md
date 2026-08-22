# skill-tool 快速入门

这份文档是写给会调用 `skill-tool` 的 AI agent 看的。

当前版本：`a0.1`


## 工具用途

`skill-tool` 是一个本地命令行工具，用来管理微信文章工程。

它可以用于：

- 输出当前工具版本
- 新建文章工程
- 获取某篇文章的本地工程路径
- 查看可用模板
- 将模板拉到本地文章工程
- 在上传前压缩超大图片
- 上传文章内容、元数据和图片
- 生成预览二维码给用户查看


## 环境变量

工具会读取以下环境变量：

- `WX_ARTICLE_REPO_DIR`
  本地文章工程的根目录。
- `WX_ARTICLE_HOST`
  后端服务地址。

如果使用 `debug/` 下编译好的二进制，它会按下面顺序查找 `.env`：

- 当前工作目录
- 可执行文件所在目录
- 可执行文件所在目录的上一级目录


## 本地工程结构

执行 `init` 后，文章工程结构如下：

```text
{WX_ARTICLE_REPO_DIR}/{article_uuid}/
  images/
  metadata.json
  article.html
```

说明：

- `metadata.json`
  正式文章的元数据。
- `article.html`
  正式文章内容。
- `images/`
  本地图片目录。

执行 `fetch-template` 后，工程目录中还会额外出现：

```text
{WX_ARTICLE_REPO_DIR}/{article_uuid}/
  metadata.example.json
  example.html
```

说明：

- `metadata.example.json`
  模板元数据。
- `example.html`
  模板示例内容。


## 命令速查

### 1. 查看工具版本

```powershell
skill-tool --version
```

适用场景：

- 需要确认当前二进制版本
- 需要判断当前环境是否已经更新到预期版本

注意：

- 当前版本由仓库中的 `VERSION` 文件统一定义


### 2. 新建文章工程

```powershell
skill-tool init
```

适用场景：

- 用户要开始一篇新文章

注意：

- 这个命令只会初始化正式文章所需文件
- 不会初始化模板相关文件


### 3. 获取文章工程路径

```powershell
skill-tool work-dir -a {article_uuid}
```

适用场景：

- 已知文章 id，但需要先拿到本地工程路径
- 后续要查看或操作该文章目录下的文件

返回内容通常包括：

- `project_dir`
- `metadata_path`
- `article_html_path`
- `images_dir`
- `metadata.example.json` 路径
- `example.html` 路径


### 4. 查看模板列表

```powershell
skill-tool templates-list
```

适用场景：

- 用户想先看看有哪些模板可以选

注意：

- 不要默认替用户选模板
- 先拿模板列表，再和用户确认是否需要、需要哪一个


### 5. 拉取模板

```powershell
skill-tool fetch-template -a {article_uuid} -t {template_uuid}
```

适用场景：

- 用户已经确认要使用某个模板

参数说明：

- `-a --article`
  本地文章工程 id
- `-t --template`
  后端模板 id

注意：

- 这个命令只会写入 `metadata.example.json` 和 `example.html`
- 不会修改 `metadata.json`
- 不会修改 `article.html`


### 6. 压缩单张图片

```powershell
skill-tool compress -a {article_uuid} -n {image_name}
```

适用场景：

- 某张图片超过 `10MB`
- `upload` 被图片体积拦住

参数说明：

- `-a --article`
  本地文章工程 id
- `-n --name`
  图片文件名，只能传 `images/` 目录下的直接文件名

注意：

- 不要传嵌套路径
- 这个命令会覆盖原图
- 当前只支持 `jpg`、`jpeg`、`png`

压缩策略：

- 只有图片大于 `10MB` 时，才会真正执行压缩
- 如果图片小于等于 `10MB`，命令会返回成功，但不会改文件
- `jpeg/jpg`
  先保留原尺寸，再按较温和的质量档位逐步压缩
- 如果还不够，再依次缩到 `95%`、`90%`、`85%`
- `png`
  先做无损压缩
- 如果还不够，再依次缩到 `95%`、`90%`、`85%`
- 目标体积尽量控制在 `9.5MB` 左右，给上传留余量


### 7. 上传文章

```powershell
skill-tool upload -a {article_uuid}
```

适用场景：

- 用户已经确认正式文章可以上传

注意：

- `article.html` 不能为空
- `metadata.json.title` 不能为空
- `images/` 中只要有任意一张图片超过 `10MB`，上传会在真正发请求前直接中止
- 如果被体积拦住，先对对应图片执行 `compress`

会上传：

- 正式文章元数据
- 正式文章 HTML
- 图片

不会上传：

- `metadata.example.json`
- `example.html`


### 8. 生成预览二维码

```powershell
skill-tool preview -a {article_uuid}
```

适用场景：

- 用户想预览当前线上文章效果

注意：

- 会在 `.cache` 目录下生成二维码图片
- 如果条件允许，直接在对话中把二维码展示给用户看


## 返回字段约定

所有命令都返回 JSON。

常见字段：

- `ok`
  命令是否成功。
- `message`
  当前状态的自然语言说明。
- `next_commands`
  如果用户确认继续，接下来可能会用到的命令列表。
- `suggested_prompt`
  给 agent 的自然语言提示。不是所有命令都有。
- `error`
  当 `ok` 为 `false` 时的错误信息。


## Agent 使用规则

- 不要替用户决定是否使用模板
- 不要替用户决定是否上传
- 不要替用户决定是否展示预览
- 如果命令返回了 `next_commands`，把它们当作“可能的下一步选项”，不要当作必须执行的指令
- 如果用户明确要看预览，并且已经生成二维码，尽量直接把二维码展示在对话里


## 一条典型流程

1. 如有需要，可先执行 `skill-tool --version` 确认当前工具版本
2. 执行 `skill-tool init`
3. 如有需要，可先执行 `skill-tool work-dir -a {article_uuid}` 获取本地工程路径
4. 询问用户是否需要模板
5. 如果需要模板，执行 `skill-tool templates-list`
6. 用户确认模板后，执行：
   `skill-tool fetch-template -a {article_uuid} -t {template_uuid}`
7. 在本地准备正式文章内容与元数据
8. 如果某张图片过大，执行：
   `skill-tool compress -a {article_uuid} -n {image_name}`
9. 用户确认可以上传后，执行：
   `skill-tool upload -a {article_uuid}`
10. 用户确认要预览后，执行：
   `skill-tool preview -a {article_uuid}`
