# GoScheduler 1.6 - 定时任务管理系统
[![Downloads](https://img.shields.io/github/downloads/peiban666/GoScheduler/total.svg)](https://github.com/peiban666/GoScheduler/releases)
[![license](https://img.shields.io/github/license/peiban666/GoScheduler.svg)](https://github.com/peiban666/GoScheduler/blob/main/LICENSE)
[![Version](https://img.shields.io/badge/version-1.6-blue.svg)](https://github.com/peiban666/GoScheduler)

# 项目简介
使用Go语言开发的轻量级定时任务集中调度和管理系统，支持http、shell任务

项目的基础代码来自 https://github.com/ouqiang/gocron  
gocron 是一个很好的项目，但遗憾的是，这个项目已经四年多没有更新了  
基于 gocron，我做了 goscheduler，计划长期维护，欢迎大家提issue或者加入开发  
gocron、goscheduler 使用的都是 MIT LICENSE

本仓库基于 [gaggad/goscheduler](https://github.com/gaggad/goscheduler) 扩展，
继续使用与上游一致的 **MIT License**，保留上游版权声明。
当前版本为 **1.6**，本次变更见下方更新日志及 [CHANGELOG.md](CHANGELOG.md)。

## 功能特性
* Web界面管理定时任务
* crontab时间表达式, 精确到秒
* 任务执行失败可重试
* 任务执行超时, 强制结束
* 任务依赖配置, A任务完成后再执行B任务
* 账户权限控制
* 任务类型
    * shell任务
  > 在任务节点上执行shell命令, 支持任务同时在多个节点上运行
    * HTTP任务
  > 访问指定的URL地址, 由调度器直接执行, 不依赖任务节点
* 查看任务执行结果日志
* 任务执行结果通知, 支持邮件、Slack、Webhook

### 可视化执行时间

新增或编辑主任务时，在“执行时间”中选择：

- **每 X 分钟**：支持 1–60 分钟，从每小时 `00` 分起计算，秒固定为 `00`。例如每 5 分钟执行于 `00、05、10…55` 分，保存为 `0 */5 * * * *`，不从保存时间起算。
- **每 X 小时**：支持 1–24 小时，从每天 `00` 时起计算，可选择第几分钟执行。例如每 2 小时、00 分执行于 `00:00、02:00…22:00`；填 1 即原来的每小时模式。
- **每天**：使用时间选择器添加多个时间点，例如 `03:00`、`07:30`、`20:00`。
- **高级 Cron**：保留原有六字段表达式和 `@every` 等写法，多条日历规则每行一个，`@every` 间隔规则单独使用。

多个时间点仍属于同一条任务，统一启停、编辑、重试和查看日志；重合的调度规则在同一时刻只触发一次。
执行时间按调度器所在时区计算，编辑页和列表展示可读的执行时间，不额外展示自动生成的原始规则或悬停提示；调度规则仍正常保存和执行。高级模式保留表达式输入。
旧的常见每日/每小时表达式会自动回填时间选择器，复杂表达式保持在高级模式。
已有 `@every Nm`（1–60 分钟）会自动回填，并按同样的 `00` 分基准执行，不需要批量修改任务数据；再次保存时写入 Cron 步长规则。重启、启用和保存任务不改变日历时间点。例如 `21:07:17` 启用每 5 分钟，下次为 `21:10:00`。
分钟步长每小时重置，小时步长每天重置：每 7 分钟为 `00、07…56` 分，`56` 分后下次是下小时 `00` 分（相隔 4 分钟）；每 5 小时为 `00、05、10、15、20` 时，`20` 时后下次是次日 `00` 时（相隔 4 小时）。选择器会提示这些边界差异。
旧的大于 60 分钟的 `@every Nm` 及高级模式 `@every 10s`、`@every 60s`、`@every 1h` 等保留原间隔语义，在高级模式编辑，不自动改写。

例如每天 `03:00`、`07:30`、`20:00` 保存为：

```text
0 0 3,20 * * *
0 30 7 * * *
```

### 快捷复制任务

- 点击任务操作区的 **复制**，可复制单个任务，默认目标为原分组。
- 勾选多条任务，点击顶部 **复制到分组**；支持已有分组、未分组和「＋ 新建分组」。
- 原任务保留，副本使用新 ID、名称自动加“副本”，复制执行时间、命令、请求体、节点、重试和通知配置。
- **副本默认停用**，检查配置后再手动启用，避免重复执行。
- 同一批选中任务间的依赖指向新副本；未选中的依赖仍指向原任务。若需要完整复制主/子任务关系，请将关联任务一起勾选。
- 批量复制采用事务，任何创建或节点关联失败都会回滚，不留下半批副本。

为了兼容现有数据库，无需迁移表结构；生成的调度规则总长度沿用 `spec` 字段的 64 字符上限。
相同分钟的时间点会自动合并，重复时间点会去重；超过长度时前后端都会提示。
已有重复任务不会自动合并或删除：先给保留的任务添加全部时间点，验证后再停用其余重复任务。
部署时需同时更新后端和前端静态资源。
前端逻辑测试：在 `web/vue` 下运行 `npm test`（Node.js 18+）。
生产构建后运行 `npm run test:assets`，校验图标字体路径在根路径和子路径部署时都正确。

### 任务分组

- 首页默认按分组折叠展示，点击组名后才加载组内任务；每组独立分页，分组数量覆盖全部任务，不受第一页 20 条的限制。
- 新增或编辑任务、批量移入分组的下拉框底部都有「＋ 新建分组」。创建成功后自动选中，也可以在首页直接新建空分组。未分配的任务归入“未分组”。
- 展开分组、勾选多个任务，再点击“移入分组”，可以将这些任务放入同一组。也可以移回“未分组”。
- 支持按分组、任务名称、节点、执行方式和状态筛选；“组内新增”会预选当前分组。
- 沿用已有 `tag` 字段保存归属，在现有 `setting` 表记录独立创建的组名，无需迁移数据库；已有标签自然成为分组。组名最多 32 个字符，独立创建的空组会保留。
- 批量移动只更新分组字段，不重新注册调度器，不改变执行时间、启停状态或任务依赖。普通用户只读，管理员可以编辑和移动。
- 组标题提供“重命名”，支持有任务的分组和空分组；改名同步更新组内全部任务的归属，不改变任务 ID、执行时间或启停状态。重名会提示修改，不会自动合并分组。“未分组”名称固定。
- 组标题提供“删除分组”，默认仅删分组并将任务移到“未分组”；也可选择同时删除组内全部任务并移除其后续调度。确认窗口显示整个分组的任务数量，不受搜索或分页影响，数量发生变化时需重新确认。历史任务日志保留，已运行的实例不会被强制终止。“未分组”是固定入口，不提供删除。

### 多 Webhook 与机器人加签

- 在“系统管理 → 通知配置 → Webhook”新建多条有名称的通知地址，选择通用 Webhook、钉钉机器人或飞书机器人；机器人启用了签名校验时，打开“加签”并填入对应密钥。已有“默认 Webhook”同样支持修改名称。
- 任务选择 Webhook 通知后，可选择一个或多个通知地址，不同任务独立配置。旧任务未填写接收者 ID 时继续使用已有默认地址，不会自动发送到新建地址。
- “测试发送”发送一条真实测试通知，发送前明确确认；可测试未保存的修改，但不会自动保存。手动测试只发送一次，检查 HTTP 状态及机器人业务结果码，不把 HTTP 200 当作成功。
- 正被任务引用的 Webhook 不允许删除，需先修改相关任务的通知设置。
- 钉钉使用毫秒时间戳，在 URL 查询参数附加 `timestamp`、`sign`；飞书使用秒时间戳，在 JSON 消息中自动加入 `timestamp`、`sign`。每次重试重新生成签名，并检查平台的业务结果码。
- 提供各平台默认文本消息模板，也可以自定义 JSON。切换平台后检查消息格式；时间戳和签名由后端生成，不需要写在模板里。
- 签名密钥保存后不回显。留空保留同一平台的已有密钥；切换平台需重新填写，关闭加签后可明确清除密钥。密钥不放入源代码、请求日志或发送失败日志；保存在现有 `setting` 表，部署环境仍需保护数据库及备份访问权限。
- 旧的通用 Webhook 配置仍按原方式发送。新增配置复用现有设置表，无需迁移；部署时同时更新后端与前端静态资源。

### 当前 Windows 工作区运行

- `Start-Production.ps1` 启动真实 Go 服务（`--env prod`）及本机 MariaDB，不启动界面预览接口。服务监听 `127.0.0.1:5926`，数据库监听 `127.0.0.1:13306`。
- 当前数据保存在 `.runtime/db`，配置位于 `.runtime/conf/app.ini`；初始管理员登录信息位于 `.runtime/ADMIN.txt`。首次登录后请修改密码。
- `.runtime` 目录已限制本机文件访问，数据库配置和管理员登录信息不提交 Git。备份时应保护数据库及包含通知地址、签名密钥的备份文件。
- 当前工作区的启动脚本使用已初始化的本机数据库；迁移到其他电脑时按下方安装流程部署 Go 服务和数据库，不复制旧预览文件当作生产数据。

### 手机与窄屏布局

- 导航不再使用固定百分比列压缩文字；窄屏时账户及品牌在第一行，导航入口自动排在下一行。
- 小于 768 像素时，侧栏变为横向页签，筛选项和操作按钮自动换行；表格仅在自己的区域内横向滚动，不撑宽整个页面。
- 手机登录框按屏幕宽度展开，输入框占满可用区域；任务编辑、通知表单和分组弹窗也限制在屏幕范围内。

### 截图
![流程图](https://raw.githubusercontent.com/gaggad/goscheduler/master/assets/screenshot/scheduler.png)
![任务](https://raw.githubusercontent.com/gaggad/goscheduler/master/assets/screenshot/task.png)
![Slack](https://raw.githubusercontent.com/gaggad/goscheduler/master/assets/screenshot/notification.png)

### 支持平台
> Windows、Linux、Mac OS

### 环境要求
>  MySQL


## 下载
[releases](https://github.com/peiban666/GoScheduler/releases)

### GitHub Actions 自动发布

`.github/workflows/release.yml` 会在推送版本标签时自动测试、构建和发布软件包；
也可在 Actions → **Release binaries** → **Run workflow** 中输入已有标签（例如 `v1.6`）补发。
手动补发不移动或重写标签，严格构建该标签指向的源代码。

- 提供 Windows、Linux、macOS 的 amd64 主程序与任务节点裸文件，及对应 `.zip` / `.tar.gz`。
- 主程序已内嵌生产前端；前端测试、生产资源校验及后端测试通过后才发布。
- 压缩包附带 MIT 许可证、README、更新日志及保留数据的升级说明。
- 附带 `SHA256SUMS.txt` 及 `BUILD-INFO.json`，记录校验值和构建来源。
- 发布文件只包含程序及明确列出的文档，不包含本机数据库、配置或通知密钥。
- 前端通过 `npm ci` 安装锁定依赖；旧 `v1.6` 标签没有 npm 锁文件时采用发布工具中的兼容锁文件，
  保持应用源代码标签不变，并在构建信息中记录依赖版本和锁文件校验值。

## 版本升级
### 从上游版本升级并保留数据

本定制版沿用当前上游的数据库表结构：可视化执行时间保存在原 `task.spec`，
分组归属沿用 `task.tag`，空分组和新增 Webhook 配置保存在原 `setting` 表。
升级不需要重新创建数据库、导入任务或重置管理员。旧任务的 ID、依赖关系、节点、
启停状态、用户及密码、执行日志和通知配置继续从原数据库读取。
原有标签作为分组显示，原有通用 Webhook 作为“默认 Webhook”使用。
对于版本标记仍为 1.5、但已包含 `request_body` 字段的上游安装，
1.6 升级时会检查并跳过重复建列，保留已有 HTTP 请求体数据。

建议按以下顺序升级：

1. 核对旧程序版本及表结构。本说明适用于与当前上游表结构一致的安装；
   更早版本先在独立数据库副本验证其原有升级链，尤其上游 v1.2 的迁移不受支持。
2. 停止旧调度器并确认运行中的任务已结束，再备份原数据库和整个 `conf` 目录。
   使用数据库逻辑备份或数据库支持的一致性快照，并验证备份可恢复；
   数据库运行时直接复制其数据目录不等同于一致性备份。
3. 保留原 `conf/app.ini`、`conf/install.lock`、`conf/.version`，以及配置引用的证书等文件。
   数据库地址、库名和表前缀必须继续指向原来的数据，不覆盖为本工作区的配置。
4. 备份旧可执行文件，用包含最新前端静态资源的本定制版替换调度器程序，
   保持原配置目录布局启动。不要进入首次安装流程，也不要同时运行新旧调度器，
   以免重复执行任务。本次新增功能未改变任务节点协议，原节点可继续使用。
5. 检查任务数量和 ID、用户登录、节点关联、通知设置及下次执行时间。
   启用的任务会恢复调度，日历分钟/小时规则保持原有整点基准；高级固定时长规则在重启后重新计算基准；
   正式切换前宜先在数据库副本验证，隔离任务执行和通知外发。

本工作区的 `Start-Production.ps1` 及 `.runtime` 属于当前电脑的独立运行环境，
不是上游安装的升级数据包；不要用这里的空库或配置覆盖用户原有数据库。
若迁移到新机器，应恢复原数据库备份并保留原配置，而不是重新安装生成新数据。
如需回退，应配套使用升级前的程序、配置及数据库备份；
本定制版新保存的多行调度规则不保证能被上游旧程序识别。

### 替换可执行文件
备份数据库及配置后，按上方步骤替换可执行文件；保留原配置和原数据库。


## 安装

###  二进制安装
1. 解压压缩包   
2. `cd 解压目录`
3. 启动
* 调度器启动
    * Windows: `goscheduler.exe web`
    * Linux、Mac OS:  `./goscheduler web`
* 任务节点启动, 默认监听0.0.0.0:5921
    * Windows:  `goscheduler-node.exe`
    * Linux、Mac OS:  `./goscheduler-node`
4. 浏览器访问 http://localhost:5920

### 源码安装

- 安装 Go（建议使用支持当前依赖的较新版本）
- `git clone https://github.com/peiban666/GoScheduler.git`
- `cd GoScheduler`
- 编译 `make build`；仓库已包含内嵌前端，修改界面后须重新构建并嵌入静态资源
- 启动
    * goscheduler `./bin/goscheduler web`
    * goscheduler-node `./bin/goscheduler-node`

不使用 Make 时，可在仓库根目录执行：

```shell
go build -o bin/goscheduler ./cmd/goscheduler
go build -o bin/goscheduler-node ./cmd/node
```

修改前端后，先在 `web/vue` 下安装依赖并构建，再回到仓库根目录嵌入资源：

```shell
cd web/vue
npm ci
npm test
NODE_OPTIONS=--openssl-legacy-provider npm run build
npm run test:assets
cd ../..
go run github.com/rakyll/statik -src=web/vue/dist -dest=internal -f
go test ./...
go build -o bin/goscheduler ./cmd/goscheduler
```

以上 `NODE_OPTIONS=...` 写法适用于 Bash；PowerShell 中先执行
`$env:NODE_OPTIONS='--openssl-legacy-provider'`，再运行 `npm run build`。


### docker

上游镜像不包含本仓库的 1.6 改动。部署本版本应从本仓库构建镜像，
并复用原数据库及持久化配置：

```shell
docker build -t goscheduler:1.6 .
docker run --name goscheduler --link mysql:db -p 5920:5920 -v /your/goscheduler/conf:/app/conf goscheduler:1.6
```

配置: /app/conf/app.ini

日志: /app/log/scheduler.log

镜像不包含goscheduler-node, goscheduler-node需要和具体业务一起构建


### 开发

1. 安装 Go、Node.js 18+、Yarn（或 npm）；较新 Node.js 构建旧版 Webpack 时需设置 `NODE_OPTIONS=--openssl-legacy-provider`
2. 安装前端依赖 `make install-vue`
3. 启动goscheduler, goscheduler-node `make run`
4. 启动node server `make run-vue`, 访问地址 http://localhost:8080

访问http://localhost:8080, API请求会转发给goscheduler

`make` 编译

`make run` 编译并运行

`make package` 打包
> 生成当前系统的压缩包 goscheduler-v1.6-darwin-amd64.tar.gz goscheduler-node-v1.6-darwin-amd64.tar.gz

`make package-all` 生成Windows、Linux、Mac的压缩包

### 命令

* goscheduler
    * -v 查看版本

* goscheduler web
    * --host 默认0.0.0.0
    * -p 端口, 指定端口, 默认5920
    * -e 指定运行环境, dev|test|prod, dev模式下可查看更多日志信息, 默认prod
    * -h 查看帮助
* goscheduler-node
    * -allow-root *nix平台允许以root用户运行
    * -s ip:port 监听地址
    * -enable-tls 开启TLS
    * -ca-file   CA证书文件   
    * -cert-file 证书文件
    * -key-file  私钥文件
    * -h 查看帮助
    * -v 查看版本

## To Do List

## 程序使用的组件
* Web框架 [Macaron](http://go-macaron.com/)
* 定时任务调度 [Cron](https://github.com/robfig/cron)
* ORM [Xorm](https://github.com/go-xorm/xorm)
* UI框架 [Element UI](https://github.com/ElemeFE/element)
* 依赖管理 [Govendor](https://github.com/kardianos/govendor)
* RPC框架 [gRPC](https://github.com/grpc/grpc)

## 反馈
提交[issue](https://github.com/peiban666/GoScheduler/issues/new)

## ChangeLog

调度规则更新 — 2026-10-09
--------
* 每 X 分钟从每小时 00 分起执行，不再按保存时间计算；已有 1–60 分钟规则同步生效。
* 新增每 X 小时，支持从每天 00 时起按 1–24 小时步长执行，并可选择分钟。
* 增加实际时间点预览、跨小时/跨天边界提示和前后端回归测试，兼容旧五字段日历表达式。

v1.6 — 2026-10-07
--------
* 新增可视化执行时间：每 X 分钟、每小时、每天多个时间点，以及高级 Cron 模式；同一任务无需按时间拆成多条。
* 修复每 X 分钟规则跨小时的间隔及下次执行时间，分钟规则的实际触发时间对齐到整秒 `00`。
* 新增任务分组、折叠列表、组内分页、批量移动，以及各选择器底部「＋ 新建分组」。
* 支持分组重命名、删除；删除时可选择保留任务并移到“未分组”，或同时删除组内任务。
* 新增多 Webhook、任务独立选择通知地址、钉钉和飞书机器人加签、真实测试发送；默认 Webhook 名称可修改。
* 修复通知类型保存回显、管理页面接口兼容、图标字体路径与生产静态资源加载。
* 新增桌面窄屏及手机版自适应：导航换行、侧栏页签、筛选和按钮换行、手机登录框及表单展开。
* 使用真实 Go 服务和数据库，移除界面预览提示；新增前后端自动化测试及生产资源校验。
* 保持上游数据库结构兼容，新增无损升级说明；修复旧版本标记导致重复添加 `request_body` 字段的问题。
* 主程序、任务节点版本统一为 1.6；继续使用上游 MIT License，保留原版权声明。
* Docker 从本仓库源码和内嵌前端构建，不再克隆上游旧版；发布包包含 MIT 许可证及更新日志。

v1.5.5
--------
* 优化ui

v1.5.4
--------
* HTTP POST 请求体存储到数据库

v1.5
--------
* 前端使用Vue+ElementUI重构
* 任务通知
    * 新增WebHook通知
    * 自定义通知模板
    * 匹配任务执行结果关键字发送通知
* 任务列表页显示任务下次执行时间

v1.4
--------
* HTTP任务支持POST请求
* 后台手动停止运行中的shell任务
* 任务执行失败重试间隔时间支持用户自定义
* 修复API接口调用报403错误

v1.3
--------
* 支持多用户登录
* 增加用户权限控制


v1.2.2
--------
* 用户登录页增加图形验证码
* 支持从旧版本升级
* 任务批量开启、关闭、删除
* 调度器与任务节点支持HTTPS双向认证
* 修复任务列表页总记录数显示错误

v1.1
--------

* 任务可同时在多个节点上运行
* *nix平台默认禁止以root用户运行任务节点
* 子任务命令中增加预定义占位符, 子任务可根据主任务运行结果执行相应操作
* 删除守护进程模块
* Web访问日志输出到终端
